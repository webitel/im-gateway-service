package markdown

import (
	"errors"
	"reflect"
	"testing"

	"github.com/webitel/im-gateway-service/internal/domain/model"
)

func Test_Parse(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantText string
		want     []model.Entity
		wantErr  bool
	}{
		// ===== Plain text (no formatting) =====
		{
			name:     "plain text only",
			raw:      "hello world",
			wantText: "hello world",
			want:     []model.Entity{},
			wantErr:  false,
		},

		// ===== Bold: **text** =====
		{
			name:     "bold text",
			raw:      "**bold**",
			wantText: "bold",
			want: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 0,
					Length: 4,
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "bold with surrounding text",
			raw:      "hello **bold** world",
			wantText: "hello bold world",
			want: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 6,
					Length: 4,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Italic: *text* =====
		{
			name:     "italic text",
			raw:      "*italic*",
			wantText: "italic",
			want: []model.Entity{
				{
					Type:   model.EntityTypeItalic,
					Offset: 0,
					Length: 6,
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "italic with surrounding text",
			raw:      "hello *italic* world",
			wantText: "hello italic world",
			want: []model.Entity{
				{
					Type:   model.EntityTypeItalic,
					Offset: 6,
					Length: 6,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Strikethrough: ~~text~~ =====
		{
			name:     "strikethrough text",
			raw:      "~~strike~~",
			wantText: "strike",
			want: []model.Entity{
				{
					Type:   model.EntityTypeStrikethrough,
					Offset: 0,
					Length: 6,
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "strikethrough with surrounding text",
			raw:      "hello ~~strike~~ world",
			wantText: "hello strike world",
			want: []model.Entity{
				{
					Type:   model.EntityTypeStrikethrough,
					Offset: 6,
					Length: 6,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Inline code: `text` =====
		{
			name:     "inline code",
			raw:      "`code`",
			wantText: "code",
			want: []model.Entity{
				{
					Type:   model.EntityTypeCode,
					Offset: 0,
					Length: 4,
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "inline code with surrounding text",
			raw:      "hello `code` world",
			wantText: "hello code world",
			want: []model.Entity{
				{
					Type:   model.EntityTypeCode,
					Offset: 6,
					Length: 4,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Code blocks: ```text``` =====
		{
			name:     "code block",
			raw:      "```block```",
			wantText: "block",
			want: []model.Entity{
				{
					Type:   model.EntityTypePre,
					Offset: 0,
					Length: 5,
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "code block with surrounding text",
			raw:      "hello ```block``` world",
			wantText: "hello block world",
			want: []model.Entity{
				{
					Type:   model.EntityTypePre,
					Offset: 6,
					Length: 5,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Links: [text](url) =====
		{
			name:     "link with http scheme",
			raw:      "[link](http://example.com)",
			wantText: "link",
			want: []model.Entity{
				{
					Type:   model.EntityTypeLink,
					Offset: 0,
					Length: 4,
					Value:  strPtr("http://example.com"),
				},
			},
			wantErr: false,
		},
		{
			name:     "link with https scheme",
			raw:      "[link](https://example.com)",
			wantText: "link",
			want: []model.Entity{
				{
					Type:   model.EntityTypeLink,
					Offset: 0,
					Length: 4,
					Value:  strPtr("https://example.com"),
				},
			},
			wantErr: false,
		},
		{
			name:     "link with mailto scheme",
			raw:      "[email](mailto:test@example.com)",
			wantText: "email",
			want: []model.Entity{
				{
					Type:   model.EntityTypeLink,
					Offset: 0,
					Length: 5,
					Value:  strPtr("mailto:test@example.com"),
				},
			},
			wantErr: false,
		},
		{
			name:     "link with surrounding text",
			raw:      "see [link](https://example.com) here",
			wantText: "see link here",
			want: []model.Entity{
				{
					Type:   model.EntityTypeLink,
					Offset: 4,
					Length: 4,
					Value:  strPtr("https://example.com"),
				},
			},
			wantErr: false,
		},

		// ===== Multiple entities =====
		{
			name:     "multiple formatting spans",
			raw:      "**bold** *italic* ~~strike~~",
			wantText: "bold italic strike",
			want: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 0,
					Length: 4,
					Value:  nil,
				},
				{
					Type:   model.EntityTypeItalic,
					Offset: 5,
					Length: 6,
					Value:  nil,
				},
				{
					Type:   model.EntityTypeStrikethrough,
					Offset: 12,
					Length: 6,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Nested formatting within depth limit =====
		{
			name:     "nested bold in italic",
			raw:      "*italic **bold** italic*",
			wantText: "italic bold italic",
			want: []model.Entity{
				{
					Type:   model.EntityTypeItalic,
					Offset: 0,
					Length: 18,
					Value:  nil,
				},
				{
					Type:   model.EntityTypeBold,
					Offset: 7,
					Length: 4,
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "nested italic in bold",
			raw:      "**bold *italic* bold**",
			wantText: "bold italic bold",
			want: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 0,
					Length: 16,
					Value:  nil,
				},
				{
					Type:   model.EntityTypeItalic,
					Offset: 5,
					Length: 6,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Disallowed link schemes should error =====
		{
			name:     "link with javascript scheme rejected",
			raw:      "[link](javascript:alert('xss'))",
			wantText: "",
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "link with data scheme rejected",
			raw:      "[link](data:text/html,<script>)",
			wantText: "",
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "link with relative path rejected",
			raw:      "[link](../../../etc/passwd)",
			wantText: "",
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "link with empty URL treated as literal",
			raw:      "[link]()",
			wantText: "[link]()",
			want:     []model.Entity{},
			wantErr:  false,
		},

		// ===== Input size limit =====
		{
			name:     "input exceeds MaxInputBytes rejected",
			raw:      string(make([]byte, MaxInputBytes+1)),
			wantText: "",
			want:     nil,
			wantErr:  true,
		},

		// ===== Entity count limit =====
		{
			name: "more than MaxEntities rejected",
			raw: func() string {
				var s string
				for range MaxEntities + 1 {
					s += "`x` "
				}

				return s
			}(),
			wantText: "",
			want:     nil,
			wantErr:  true,
		},

		// ===== Malformed/unclosed markdown treated as literal =====
		{
			name:     "unclosed bold treated as literal",
			raw:      "hello **unclosed",
			wantText: "hello **unclosed",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "unclosed italic treated as literal",
			raw:      "hello *unclosed",
			wantText: "hello *unclosed",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "unclosed strikethrough treated as literal",
			raw:      "hello ~~unclosed",
			wantText: "hello ~~unclosed",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "unclosed inline code treated as literal",
			raw:      "hello `unclosed",
			wantText: "hello `unclosed",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "unclosed code block treated as literal",
			raw:      "hello ```unclosed",
			wantText: "hello ```unclosed",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "unclosed link text bracket treated as literal",
			raw:      "hello [unclosed",
			wantText: "hello [unclosed",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "link without closing paren treated as literal",
			raw:      "hello [text](unclosed",
			wantText: "hello [text](unclosed",
			want:     []model.Entity{},
			wantErr:  false,
		},

		// ===== Content inside code spans is literal (no reinterpretation) =====
		{
			name:     "markdown inside inline code not parsed",
			raw:      "`**not bold**`",
			wantText: "**not bold**",
			want: []model.Entity{
				{
					Type:   model.EntityTypeCode,
					Offset: 0,
					Length: 12,
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "markdown inside code block not parsed",
			raw:      "```**not bold**```",
			wantText: "**not bold**",
			want: []model.Entity{
				{
					Type:   model.EntityTypePre,
					Offset: 0,
					Length: 12,
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "link syntax inside code span not parsed",
			raw:      "`[not a link](http://example.com)`",
			wantText: "[not a link](http://example.com)",
			want: []model.Entity{
				{
					Type:   model.EntityTypeCode,
					Offset: 0,
					Length: 32,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Multi-byte UTF-8 (Cyrillic, emoji) produces correct BYTE offsets =====
		{
			name:     "Cyrillic text with bold",
			raw:      "Привет **жирный** мир",
			wantText: "Привет жирный мир",
			want: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 13, // "Привет " = П(2)+р(2)+и(2)+в(2)+е(2)+т(2)+space(1) = 13 bytes
					Length: 12, // "жирный" = ж(2)+и(2)+р(2)+н(2)+ы(2)+й(2) = 12 bytes
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "emoji in text",
			raw:      "hello **😀emoji😀** world",
			wantText: "hello 😀emoji😀 world",
			want: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 6,
					Length: 13, // 😀=4 bytes, e=1, m=1, o=1, j=1, i=1, 😀=4 bytes = 4+5+4 = 13
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Boundary cases =====
		{
			name:     "single character bold",
			raw:      "**x**",
			wantText: "x",
			want: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 0,
					Length: 1,
					Value:  nil,
				},
			},
			wantErr: false,
		},
		{
			name:     "empty text after bold at end",
			raw:      "**text**",
			wantText: "text",
			want: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 0,
					Length: 4,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Complex nested scenarios =====
		{
			name:     "bold with italic and strikethrough",
			raw:      "**bold *italic* ~~strike~~ more**",
			wantText: "bold italic strike more",
			want: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 0,
					Length: 23,
					Value:  nil,
				},
				{
					Type:   model.EntityTypeItalic,
					Offset: 5,
					Length: 6,
					Value:  nil,
				},
				{
					Type:   model.EntityTypeStrikethrough,
					Offset: 12,
					Length: 6,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Empty spans treated as literal =====
		{
			name:     "empty bold treated as literal",
			raw:      "****",
			wantText: "****",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "empty inline code treated as literal",
			raw:      "hi ``",
			wantText: "hi ``",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "empty strikethrough treated as literal",
			raw:      "~~~~",
			wantText: "~~~~",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "empty code block treated as literal",
			raw:      "``````",
			wantText: "``````",
			want:     []model.Entity{},
			wantErr:  false,
		},
		{
			name:     "empty link text treated as literal",
			raw:      "[](https://x.com)",
			wantText: "[](https://x.com)",
			want:     []model.Entity{},
			wantErr:  false,
		},

		// ===== Balanced parens in links =====
		{
			name:     "link with balanced parens in URL",
			raw:      "[x](https://en.wikipedia.org/wiki/A_(b))",
			wantText: "x",
			want: []model.Entity{
				{
					Type:   model.EntityTypeLink,
					Offset: 0,
					Length: 1,
					Value:  strPtr("https://en.wikipedia.org/wiki/A_(b)"),
				},
			},
			wantErr: false,
		},

		// ===== Code block with language tag =====
		{
			name:     "code block with language tag and newline",
			raw:      "```go\nfmt.Println()\n```",
			wantText: "fmt.Println()",
			want: []model.Entity{
				{
					Type:   model.EntityTypePre,
					Offset: 0,
					Length: 13,
					Value:  nil,
				},
			},
			wantErr: false,
		},

		// ===== Entity cap bypassed via nesting =====
		{
			name: "entity cap bypassed via nesting rejected",
			raw: func() string {
				s := "~~*"
				for range 99 {
					s += "`x`"
				}

				s += "*~~"

				return s
			}(),
			wantText: "",
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, got, err := Parse(tt.raw)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if err != nil && tt.wantErr {
				// Error case confirmed; don't check text/entities.
				return
			}

			if gotText != tt.wantText {
				t.Errorf("Parse() plainText = %q, want %q", gotText, tt.wantText)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() entities = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// strPtr returns a pointer to a string.
func strPtr(s string) *string {
	return &s
}

func Test_Parse_NestingTooDeep(t *testing.T) {
	// 4-deep nesting: link > bold > italic > strikethrough should exceed MaxNestingDepth=3.
	raw := "[**a *b ~~c~~ d* e**](https://example.com)"

	_, _, err := Parse(raw)
	if err == nil {
		t.Errorf("Parse() expected error for 4-deep nesting, got nil")
	}

	if !errors.Is(err, ErrNestingTooDeep) {
		t.Errorf("Parse() error = %v, want ErrNestingTooDeep", err)
	}

	// 3-deep nesting: link > bold > italic should succeed.
	raw = "[**a *b* c**](https://example.com)"

	_, _, err = Parse(raw)
	if err != nil {
		t.Errorf("Parse() unexpected error for 3-deep nesting: %v", err)
	}
}

func Test_validateEntitiesAgainstPlainText(t *testing.T) {
	tests := []struct {
		name      string
		plainText string
		entities  []model.Entity
		wantErr   bool
	}{
		{
			name:      "valid entity within bounds",
			plainText: "ab",
			entities: []model.Entity{
				{Type: model.EntityTypeBold, Offset: 0, Length: 2},
			},
			wantErr: false,
		},
		{
			name:      "entity exceeds plain text length",
			plainText: "ab",
			entities: []model.Entity{
				{Type: model.EntityTypeBold, Offset: 0, Length: 5},
			},
			wantErr: true,
		},
		{
			name:      "entity offset out of bounds",
			plainText: "ab",
			entities: []model.Entity{
				{Type: model.EntityTypeBold, Offset: 5, Length: 1},
			},
			wantErr: true,
		},
		{
			name:      "negative offset",
			plainText: "ab",
			entities: []model.Entity{
				{Type: model.EntityTypeBold, Offset: -1, Length: 2},
			},
			wantErr: true,
		},
		{
			name:      "zero length",
			plainText: "ab",
			entities: []model.Entity{
				{Type: model.EntityTypeBold, Offset: 0, Length: 0},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEntitiesAgainstPlainText(tt.plainText, tt.entities)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEntitiesAgainstPlainText() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil && tt.wantErr {
				if !errors.Is(err, ErrEntityBoundsCorrupted) {
					t.Errorf("validateEntitiesAgainstPlainText() error = %v, want ErrEntityBoundsCorrupted", err)
				}
			}
		})
	}
}

func Test_validateEntity_boundaries(t *testing.T) {
	p := &parser{}

	tests := []struct {
		name    string
		offset  int32
		length  int32
		value   *string
		wantErr bool
	}{
		{
			name:    "valid entity at edge of offset range",
			offset:  int32(MaxEntityOffset - 1),
			length:  1,
			value:   nil,
			wantErr: false,
		},
		{
			name:    "offset exceeds MaxEntityOffset",
			offset:  int32(MaxEntityOffset + 1),
			length:  1,
			value:   nil,
			wantErr: true,
		},
		{
			name:    "sum equals exactly 1_000_000",
			offset:  int32(MaxEntityOffset - 1),
			length:  1,
			value:   nil,
			wantErr: false,
		},
		{
			name:    "sum exceeds MaxEntityOffset",
			offset:  int32(MaxEntityOffset),
			length:  1,
			value:   nil,
			wantErr: true,
		},
		{
			name:    "value at 4096 bytes",
			offset:  0,
			length:  1,
			value:   strPtr(string(make([]byte, 4096))),
			wantErr: false,
		},
		{
			name:    "value exceeds 4096 bytes",
			offset:  0,
			length:  1,
			value:   strPtr(string(make([]byte, 4097))),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validateEntity(tt.offset, tt.length, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEntity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
