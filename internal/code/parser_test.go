package code

import (
	"reflect"
	"testing"
)

func Test_parseTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "basic title match",
			input: `it("test title", () => {`,
			want: "test title",
		},
		{
			name: "match with test object argument",
			input: `'test title', { tags: ['config', 'some-other-tag'] }`,
			want: "test title",
		},
		{
			name: "nested quotation marks",
			input: `'title "with nested" quotations', { tags: ['config', 'some-other-tag'] }`,
			want: `title "with nested" quotations`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTitle(tt.input); got != tt.want {
				t.Errorf("parseTitle() = \"%v\", want \"%v\"", got, tt.want)
			}
		})
	}
}

func Test_parseTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "no tags",
			input: `it("test title", () => {`,
			want: "",
		},
		{
			name: "multi tag",
			input: `'test title', { tags: ['tag1', 'tag2'] }`,
			want: "tag1 tag2",
		},
		{
			name: "single tag",
			input: `'title', { tags: 'tag' }`,
			want: `tag`,
		},
		{
			name: "multiline definition",
			input: `
'title "with nested" quotations', { 
  tags: [
    'tag1', 
	'tag2',
  ],
}`,
			want: `tag1 tag2`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTags(tt.input); got != tt.want {
				t.Errorf("parseTags() = \"%v\", want \"%v\"", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []TestCase
	}{
		{
			name: "basic test case match",
			input: `
it('test title', () => {
  expect(true).to.be.true
})
`,
			want: []TestCase {
				{
					Title: "test title",
					Tags: "",
				},
			},
		},
		{
			name: "parse test case with multiple tags",
			input: `
it("test title", { tags: ['@tag1', "@tag2"] }, () => {
  expect(true).to.be.true
})
`,
			want: []TestCase {
				{
					Title: "test title",
					Tags: "@tag1 @tag2",
				},
			},
		},
		{
			name: "parse test case with single tag",
			input: `
it("test title", { tags: '@tag1' }, () => {
  expect(true).to.be.true
})
`,
			want: []TestCase {
				{
					Title: "test title",
					Tags: "@tag1",
				},
			},
		},
		{
			name: "parse test case with complex test object",
			input: `
it("test title", {
  tags: [
    '@tag1', 
    '@tag2'
  ],
}, () => {
  expect(true).to.be.true
})
`,
			want: []TestCase {
				{
					Title: "test title",
					Tags: "@tag1 @tag2",
				},
			},
		},
		{
			name: "parse multiple test cases",
			input: `
it("test title1", {
  tags: [
    '@tag1', 
    '@tag2'
  ],
}, () => {
  expect(true).to.be.true
})
it("test title2", {
  "quoted key": "string",
  tags: '@tag1', 
  field1: 1,
  field2: {
	nest1: 1,
	nest2: [1, 2, 3],
  }
}, () => {
  expect(true).to.be.true
})

`,
			want: []TestCase {
				{
					Title: "test title1",
					Tags: "@tag1 @tag2",
				},
				{
					Title: "test title2",
					Tags: "@tag1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTitle() = \"%v\", want \"%v\"", got, tt.want)
			}
		})
	}
}
