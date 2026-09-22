package markdown

import (
	"errors"
	"net/url"
	"sort"
	"strings"

	"github.com/webitel/im-gateway-service/internal/domain/model"
)

// Constants that govern the markdown parser's behavior and limits.
const (
	// MaxInputBytes is a defense-in-depth backstop against pathological inputs.
	// The scanner below is now linear-time, so this is not the primary CPU-exhaustion
	// mitigation; it is sized for a realistic chat message body.
	MaxInputBytes = 16384

	// MaxNestingDepth limits the depth of nested inline formatting constructs.
	// There are exactly 4 distinct nestable delimiters (bold **, italic *,
	// strikethrough ~~, link [...](...)). Reusing the same delimiter twice in
	// one ancestor/descendant nesting chain is structurally unparseable: the
	// outer occurrence's own linear scan for its closing marker would match the
	// inner occurrence's marker first. Thus, 4 is the true structural ceiling on
	// chain depth. MaxNestingDepth is kept at 3 (below that ceiling) specifically
	// so the guard is reachable/exercised by a legitimate 4-construct chain.
	MaxNestingDepth = 3

	// MaxEntities is the maximum number of formatting spans a single parse can produce.
	// Matches the buf.validate constraint on Entity arrays (max_items: 100).
	MaxEntities = 100

	// Per-entity bounds (must match proto constraints).
	MinEntityOffset     = 0
	MaxEntityOffset     = 1_000_000
	MinEntityLength     = 1
	MaxEntityLength     = 1_000_000
	MaxEntityValueBytes = 4096
)

var (
	// Sentinel errors returned when parsing fails.
	ErrInputTooLarge        = errors.New("markdown: input exceeds maximum size")
	ErrNestingTooDeep       = errors.New("markdown: nesting depth exceeded")
	ErrDisallowedLinkScheme = errors.New("markdown: link scheme not allowed")
	ErrTooManyEntities      = errors.New("markdown: too many entities")
	ErrEntityBoundsInvalid  = errors.New("markdown: entity bounds invalid")
	// ErrEntityBoundsCorrupted is returned by the final defense-in-depth sweep in Parse
	// when a produced entity's offset/length is inconsistent with the actual plain-text
	// length. This can only mean an internal parser bug, never bad user input.
	ErrEntityBoundsCorrupted = errors.New("markdown: entity bounds inconsistent with plain text")
)

// allowedLinkSchemes is the set of URL schemes permitted in links.
// Use a map-based lookup instead of a switch to avoid exhaustive linter issues.
var allowedLinkSchemes = map[string]struct{}{
	"http":   {},
	"https":  {},
	"mailto": {},
}

// Parse converts markdown text into a plain-text string and a list of formatting entities.
// The parser:
//   - Recognizes CommonMark-style markdown: **bold**, *italic*, ~~strikethrough~~, `code`, ```pre```, [text](url)
//   - Treats malformed/unclosed markdown as literal plain text (does not error)
//   - Does not recursively parse content inside CODE or PRE spans
//   - Validates link URLs (scheme must be http, https, or mailto)
//   - Returns entity offsets as UTF-8 byte offsets into the final plain-text string
func Parse(raw string) (plainText string, entities []model.Entity, err error) {
	// Check input size before doing any parsing work (ReDoS/CPU mitigation).
	if len(raw) > MaxInputBytes {
		return "", nil, ErrInputTooLarge
	}

	p := &parser{
		raw:       raw,
		plainText: strings.Builder{},
		entities:  make([]model.Entity, 0),
	}

	if err := p.parseInline(); err != nil {
		return "", nil, err
	}

	plainText = p.plainText.String()
	entities = p.entities

	// Final backstop: entity count check independent of per-merge checks.
	if len(entities) > MaxEntities {
		return "", nil, ErrTooManyEntities
	}

	// Defense-in-depth bounds sweep.
	if err := validateEntitiesAgainstPlainText(plainText, entities); err != nil {
		return "", nil, err
	}

	// Sort by offset ascending, then length descending.
	sort.SliceStable(entities, func(i, j int) bool {
		if entities[i].Offset != entities[j].Offset {
			return entities[i].Offset < entities[j].Offset
		}

		return entities[i].Length > entities[j].Length
	})

	return plainText, entities, nil
}

type parser struct {
	raw       string
	pos       int // position in raw input
	depth     int // current nesting depth
	plainText strings.Builder
	entities  []model.Entity

	// Caching flags: each caches that a prior forward scan already searched the
	// remainder of raw for that construct's closing marker and found none. Once true,
	// later attempts at that construct type skip scanning entirely. This is sound
	// because parsing only moves forward within one parser instance (each recursive
	// call gets its own fresh parser struct with these fields defaulting to false),
	// so once a scan from position X to end-of-string finds no closing marker,
	// no later scan starting at Y>=X can find one either.
	noBoldCloser   bool
	noItalicCloser bool
	noStrikeCloser bool
	noLinkCloser   bool
}

// parseInlineContent is a helper that parses a string and returns plain text + entities
// without requiring the caller to pass the Builder. This avoids the "non-zero Builder
// copied by value" panic.
func (p *parser) parseInlineContent(content string) (string, []model.Entity, error) {
	subParser := &parser{
		raw:       content,
		pos:       0,
		depth:     p.depth,
		plainText: strings.Builder{},
		entities:  make([]model.Entity, 0),
	}

	if err := subParser.parseInline(); err != nil {
		return "", nil, err
	}

	return subParser.plainText.String(), subParser.entities, nil
}

// parseInline parses inline content up to a terminating condition.
func (p *parser) parseInline() error {
	for p.pos < len(p.raw) {
		ch := p.raw[p.pos]

		// Check for backtick (inline code) or triple-backtick (code block).
		if ch == '`' {
			if err := p.handleBacktick(); err != nil {
				return err
			}

			continue
		}

		// Check for **bold** or *italic*
		if ch == '*' {
			if err := p.handleAsterisk(); err != nil {
				return err
			}

			continue
		}

		// Check for ~~strikethrough~~
		if ch == '~' && p.peek() == '~' {
			if err := p.handleStrikethrough(); err != nil {
				return err
			}

			continue
		}

		// Check for [link](url)
		if ch == '[' {
			if err := p.handleLink(); err != nil {
				return err
			}

			continue
		}

		// Literal character.
		p.plainText.WriteByte(ch)
		p.pos++
	}

	return nil
}

// peek returns the character at pos+1, or 0 if at end.
func (p *parser) peek() byte {
	if p.pos+1 < len(p.raw) {
		return p.raw[p.pos+1]
	}

	return 0
}

// handleBacktick processes inline code (single backtick) and code blocks (triple backtick).
func (p *parser) handleBacktick() error {
	if len(p.raw)-p.pos >= 3 && p.raw[p.pos:p.pos+3] == "```" {
		// Code block: ``` ... ```
		return p.parseCodeBlock()
	}

	// Inline code: ` ... `
	return p.parseInlineCode()
}

// stripCodeFenceInfoString extracts the code block body, stripping the language tag
// (info string) and a single trailing newline.
func stripCodeFenceInfoString(rawContent string) string {
	// Check if the first line contains no whitespace (info string).
	if newlineIdx := strings.IndexByte(rawContent, '\n'); newlineIdx >= 0 {
		firstLine := rawContent[:newlineIdx]
		if !strings.ContainsAny(firstLine, " \t") {
			// Strip the info string and the newline after it.
			body := rawContent[newlineIdx+1:]
			// Strip a single trailing newline.
			body = strings.TrimSuffix(body, "\n")

			return body
		}
	}
	// No info string, but strip a single trailing newline.
	return strings.TrimSuffix(rawContent, "\n")
}

// parseCodeBlock parses a triple-backtick code block with literal content.
func (p *parser) parseCodeBlock() error {
	// Skip opening ```
	p.pos += 3

	// Find closing ```
	start := p.pos
	for p.pos < len(p.raw) {
		if len(p.raw)-p.pos >= 3 && p.raw[p.pos:p.pos+3] == "```" {
			// Found closing ```. Strip info string and trailing newline from content.
			offset := int32(p.plainText.Len())
			rawContent := p.raw[start:p.pos]
			body := stripCodeFenceInfoString(rawContent)

			// Empty-span fallthrough: if body is empty, treat the whole construct as literal.
			if body == "" {
				p.plainText.WriteString("```")
				p.plainText.WriteString(rawContent)
				p.plainText.WriteString("```")
				p.pos += 3

				return nil
			}

			// Entity-cap check: only once we know this construct is well-formed
			// and about to actually produce an entity (not eagerly at function
			// entry, which would reject valid messages on an unrelated later
			// stray marker that would never have added an entity).
			if len(p.entities) >= MaxEntities {
				return ErrTooManyEntities
			}

			p.plainText.WriteString(body)
			length := int32(len(body))

			if err := p.validateEntity(offset, length, nil); err != nil {
				return err
			}

			p.entities = append(p.entities, model.Entity{
				Type:   model.EntityTypePre,
				Offset: offset,
				Length: length,
				Value:  nil,
			})

			p.pos += 3

			return nil
		}

		p.pos++
	}

	// Unclosed code block: treat as literal using raw unstripped content.
	p.plainText.WriteString("```")
	p.plainText.WriteString(p.raw[start:])
	p.pos = len(p.raw)

	return nil
}

// parseInlineCode parses a single-backtick inline code span.
func (p *parser) parseInlineCode() error {
	// Skip opening backtick
	p.pos++

	// Find closing backtick
	start := p.pos
	for p.pos < len(p.raw) {
		if p.raw[p.pos] == '`' {
			// Found closing backtick.
			offset := int32(p.plainText.Len())
			content := p.raw[start:p.pos]

			// Empty-span fallthrough: if content is empty, treat as literal.
			if content == "" {
				p.plainText.WriteString("``")
				p.pos++

				return nil
			}

			// Entity-cap check: only once well-formedness is established.
			if len(p.entities) >= MaxEntities {
				return ErrTooManyEntities
			}

			p.plainText.WriteString(content)
			length := int32(len(content))

			if err := p.validateEntity(offset, length, nil); err != nil {
				return err
			}

			p.entities = append(p.entities, model.Entity{
				Type:   model.EntityTypeCode,
				Offset: offset,
				Length: length,
				Value:  nil,
			})

			p.pos++

			return nil
		}

		p.pos++
	}

	// Unclosed backtick: treat as literal.
	p.plainText.WriteByte('`')
	p.plainText.WriteString(p.raw[start:])
	p.pos = len(p.raw)

	return nil
}

// handleAsterisk processes **bold** or *italic*.
func (p *parser) handleAsterisk() error {
	if p.peek() == '*' {
		// Possibly **bold**
		return p.parseBold()
	}

	// Possibly *italic*
	return p.parseItalic()
}

// parseBold parses **text** for bold formatting.
func (p *parser) parseBold() error {
	// Skip if we already know there's no closing ** marker.
	if p.noBoldCloser {
		p.plainText.WriteString("**")
		p.pos += 2

		return nil
	}

	// Skip opening **
	p.pos += 2

	startPos := p.pos
	offset := int32(p.plainText.Len())

	// Find closing **
	for p.pos < len(p.raw) {
		if p.raw[p.pos] == '*' && p.peek() == '*' {
			// Found closing **: the construct is well-formed. Only now do we
			// check the nesting depth cap -- checking it eagerly at function
			// entry would hard-error on an unclosed/malformed ** that was
			// always going to fall through to literal text, contradicting the
			// "malformed markdown is literal" contract.
			if p.depth >= MaxNestingDepth {
				return ErrNestingTooDeep
			}

			p.depth++

			// Parse the content recursively (inside bold).
			content := p.raw[startPos:p.pos]

			// Parse the content into a separate result, then merge.
			plainText, subEntities, err := p.parseInlineContent(content)

			p.depth--

			if err != nil {
				return err
			}

			// Empty-span fallthrough: if plainText is empty, treat as literal.
			if plainText == "" {
				p.plainText.WriteString("****")
				p.pos += 2

				return nil
			}

			// Merge-point check: this is the ONLY entity-cap check for this
			// construct -- checking eagerly at function entry would reject
			// valid messages on an unrelated later stray marker that would
			// never have added an entity.
			if len(p.entities)+len(subEntities)+1 > MaxEntities {
				return ErrTooManyEntities
			}

			// Write the parsed text to our output.
			p.plainText.WriteString(plainText)

			// Adjust entity offsets and add them.
			for _, e := range subEntities {
				e.Offset += offset
				if err := p.validateEntity(e.Offset, e.Length, e.Value); err != nil {
					return err
				}

				p.entities = append(p.entities, e)
			}

			length := int32(len(plainText))
			if err := p.validateEntity(offset, length, nil); err != nil {
				return err
			}

			p.entities = append(p.entities, model.Entity{
				Type:   model.EntityTypeBold,
				Offset: offset,
				Length: length,
				Value:  nil,
			})

			p.pos += 2

			return nil
		}

		p.pos++
	}

	// Unclosed **: mark that we found no closer, then treat as literal.
	p.noBoldCloser = true
	p.plainText.WriteString("**")
	p.pos = startPos

	return nil
}

// parseItalic parses *text* for italic formatting.
func (p *parser) parseItalic() error {
	// Skip if we already know there's no closing * marker.
	if p.noItalicCloser {
		p.plainText.WriteByte('*')
		p.pos++

		return nil
	}

	// Skip opening *
	p.pos++

	startPos := p.pos
	offset := int32(p.plainText.Len())

	// Find closing *. A run of "**" can never be italic's own closing marker
	// (that requires a single, unpaired '*'), so any "**" encountered while
	// scanning is skipped over as a two-byte unit and scanning continues from
	// right after it. This keeps the scan a strict, monotonic single pass over
	// every byte (same guarantee as bold/strikethrough/link), which is what
	// makes noItalicCloser sound: earlier versions of this scan speculatively
	// jumped ahead looking for a matching closing "**", and on failure could
	// swallow all the way to end-of-string without ever having examined later,
	// perfectly well-formed single-'*' closers -- silently dropping them.
	for p.pos < len(p.raw) {
		if p.raw[p.pos] == '*' {
			if p.peek() == '*' {
				p.pos += 2

				continue
			}

			// This is single *, our closing marker: the construct is
			// well-formed. Only now do we check the nesting depth cap.
			if p.depth >= MaxNestingDepth {
				return ErrNestingTooDeep
			}

			p.depth++

			// Parse the content recursively (inside italic).
			content := p.raw[startPos:p.pos]

			// Parse the content into a separate result, then merge.
			plainText, subEntities, err := p.parseInlineContent(content)

			p.depth--

			if err != nil {
				return err
			}

			// Empty-span fallthrough: if plainText is empty, treat as literal.
			if plainText == "" {
				p.plainText.WriteByte('*')
				p.pos++

				return nil
			}

			// Merge-point check: this is the ONLY entity-cap check for this
			// construct.
			if len(p.entities)+len(subEntities)+1 > MaxEntities {
				return ErrTooManyEntities
			}

			// Write the parsed text to our output.
			p.plainText.WriteString(plainText)

			// Adjust entity offsets and add them.
			for _, e := range subEntities {
				e.Offset += offset
				if err := p.validateEntity(e.Offset, e.Length, e.Value); err != nil {
					return err
				}

				p.entities = append(p.entities, e)
			}

			length := int32(len(plainText))
			if err := p.validateEntity(offset, length, nil); err != nil {
				return err
			}

			p.entities = append(p.entities, model.Entity{
				Type:   model.EntityTypeItalic,
				Offset: offset,
				Length: length,
				Value:  nil,
			})

			p.pos++

			return nil
		}

		p.pos++
	}

	// Unclosed *: mark that we found no closer -- sound because the loop above
	// always advances by at least one byte and never skips past unexamined
	// content (unpaired "**" runs are consumed 2 bytes at a time, not jumped
	// over speculatively), so reaching here means every remaining byte was
	// actually inspected for a single-'*' closer.
	p.noItalicCloser = true
	p.plainText.WriteByte('*')
	p.pos = startPos

	return nil
}

// handleStrikethrough processes ~~text~~ for strikethrough.
func (p *parser) handleStrikethrough() error {
	// Skip if we already know there's no closing ~~ marker.
	if p.noStrikeCloser {
		p.plainText.WriteString("~~")
		p.pos += 2

		return nil
	}

	// Skip opening ~~
	p.pos += 2

	startPos := p.pos
	offset := int32(p.plainText.Len())

	// Find closing ~~
	for p.pos < len(p.raw) {
		if p.raw[p.pos] == '~' && p.peek() == '~' {
			// Found closing ~~: the construct is well-formed. Only now do we
			// check the nesting depth cap.
			if p.depth >= MaxNestingDepth {
				return ErrNestingTooDeep
			}

			p.depth++

			// Parse the content recursively (inside strikethrough).
			content := p.raw[startPos:p.pos]

			// Parse the content into a separate result, then merge.
			plainText, subEntities, err := p.parseInlineContent(content)

			p.depth--

			if err != nil {
				return err
			}

			// Empty-span fallthrough: if plainText is empty, treat as literal.
			if plainText == "" {
				p.plainText.WriteString("~~~~")
				p.pos += 2

				return nil
			}

			// Merge-point check: this is the ONLY entity-cap check for this
			// construct.
			if len(p.entities)+len(subEntities)+1 > MaxEntities {
				return ErrTooManyEntities
			}

			// Write the parsed text to our output.
			p.plainText.WriteString(plainText)

			// Adjust entity offsets and add them.
			for _, e := range subEntities {
				e.Offset += offset
				if err := p.validateEntity(e.Offset, e.Length, e.Value); err != nil {
					return err
				}

				p.entities = append(p.entities, e)
			}

			length := int32(len(plainText))
			if err := p.validateEntity(offset, length, nil); err != nil {
				return err
			}

			p.entities = append(p.entities, model.Entity{
				Type:   model.EntityTypeStrikethrough,
				Offset: offset,
				Length: length,
				Value:  nil,
			})

			p.pos += 2

			return nil
		}

		p.pos++
	}

	// Unclosed ~~: mark that we found no closer, then treat as literal.
	p.noStrikeCloser = true
	p.plainText.WriteString("~~")
	p.pos = startPos

	return nil
}

// handleLink processes [text](url) links.
func (p *parser) handleLink() error {
	// Skip if we already know there's no closing ] marker.
	if p.noLinkCloser {
		p.plainText.WriteByte('[')
		p.pos++

		return nil
	}

	// Skip opening [
	p.pos++

	// Find closing ]
	linkTextStart := p.pos
	linkTextEnd := -1

	for p.pos < len(p.raw) {
		if p.raw[p.pos] == ']' {
			linkTextEnd = p.pos

			break
		}

		p.pos++
	}

	if linkTextEnd == -1 {
		// Unclosed [: mark no closer, treat as literal.
		p.noLinkCloser = true
		p.plainText.WriteByte('[')
		p.pos = linkTextStart

		return nil
	}

	// Check for (url) after ]
	if linkTextEnd+1 >= len(p.raw) || p.raw[linkTextEnd+1] != '(' {
		// No opening paren: treat as literal.
		p.pos = linkTextStart - 1
		p.plainText.WriteByte('[')
		p.pos++

		return nil
	}

	// Skip ] and (
	p.pos = linkTextEnd + 2
	urlStart := p.pos

	// Find closing ), tracking balanced parens.
	urlEnd := -1
	parenDepth := 0

	for p.pos < len(p.raw) {
		if p.raw[p.pos] == '(' {
			parenDepth++
		} else if p.raw[p.pos] == ')' {
			if parenDepth == 0 {
				urlEnd = p.pos

				break
			}

			parenDepth--
		}

		p.pos++
	}

	if urlEnd == -1 {
		// Unclosed paren: treat as literal.
		p.pos = linkTextStart - 1
		p.plainText.WriteByte('[')
		p.pos++

		return nil
	}

	// Extract and validate URL.
	urlStr := p.raw[urlStart:urlEnd]
	if urlStr == "" {
		// Empty URL: treat as literal.
		p.pos = linkTextStart - 1
		p.plainText.WriteByte('[')
		p.pos++

		return nil
	}

	// Parse link text content (recursively, inside link) BEFORE validating the
	// URL's scheme, so an empty-link-text literal fallthrough below isn't
	// preempted by a link-scheme rejection for a URL that would never have
	// become an entity anyway.
	linkText := p.raw[linkTextStart:linkTextEnd]
	offset := int32(p.plainText.Len())

	// Depth check: only now, since a well-formed [text](url) structure is
	// confirmed (], (, ) all found, URL non-empty) and we're about to recurse.
	if p.depth >= MaxNestingDepth {
		return ErrNestingTooDeep
	}

	p.depth++

	plainText, subEntities, err := p.parseInlineContent(linkText)

	p.depth--

	if err != nil {
		return err
	}

	// Empty-span fallthrough: if the link text is empty, treat as literal
	// regardless of the URL's scheme.
	if plainText == "" {
		p.plainText.WriteByte('[')
		p.plainText.WriteString("](")
		p.plainText.WriteString(urlStr)
		p.plainText.WriteByte(')')
		p.pos = urlEnd + 1

		return nil
	}

	// Only validate the URL scheme once we know this will actually become a
	// LINK entity.
	if err := p.validateLinkURL(urlStr); err != nil {
		return err
	}

	// Merge-point check: this is the ONLY entity-cap check for this
	// construct.
	if len(p.entities)+len(subEntities)+1 > MaxEntities {
		return ErrTooManyEntities
	}

	// Write the parsed text to our output.
	p.plainText.WriteString(plainText)

	// Adjust entity offsets and add them.
	for _, e := range subEntities {
		e.Offset += offset
		if err := p.validateEntity(e.Offset, e.Length, e.Value); err != nil {
			return err
		}

		p.entities = append(p.entities, e)
	}

	length := int32(len(plainText))
	if err := p.validateEntity(offset, length, &urlStr); err != nil {
		return err
	}

	p.entities = append(p.entities, model.Entity{
		Type:   model.EntityTypeLink,
		Offset: offset,
		Length: length,
		Value:  &urlStr,
	})

	p.pos = urlEnd + 1

	return nil
}

// validateLinkURL checks that a URL has an allowed scheme.
func (p *parser) validateLinkURL(urlStr string) error {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return ErrDisallowedLinkScheme
	}

	scheme := strings.ToLower(parsed.Scheme)
	if _, ok := allowedLinkSchemes[scheme]; !ok {
		return ErrDisallowedLinkScheme
	}

	return nil
}

// validateEntity checks that an entity's bounds are within allowed ranges.
func (p *parser) validateEntity(offset, length int32, value *string) error {
	if offset < MinEntityOffset || offset > MaxEntityOffset {
		return ErrEntityBoundsInvalid
	}

	if length < MinEntityLength || length > MaxEntityLength {
		return ErrEntityBoundsInvalid
	}

	if offset+length > int32(MaxEntityOffset) {
		return ErrEntityBoundsInvalid
	}

	if value != nil && len(*value) > MaxEntityValueBytes {
		return ErrEntityBoundsInvalid
	}

	return nil
}

// validateEntitiesAgainstPlainText checks that all entities' offsets and lengths
// are consistent with the actual plain-text length. This is a defense-in-depth check
// that can only fail if there is an internal parser bug.
func validateEntitiesAgainstPlainText(plainText string, entities []model.Entity) error {
	textLen := int64(len(plainText))

	for _, e := range entities {
		if e.Offset < 0 || e.Length < 1 {
			return ErrEntityBoundsCorrupted
		}

		if int64(e.Offset)+int64(e.Length) > textLen {
			return ErrEntityBoundsCorrupted
		}
	}

	return nil
}
