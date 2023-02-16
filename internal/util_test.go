package internal_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/stretchr/testify/assert"
)

//nolint:funlen
func Test_JoinYAMLDocuments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		yamlDocs [][]byte
		expected string
	}{
		{
			"single document without markers",
			toByteSlice(yamlWithoutMarkers1),
			yamlWithoutMarkers1 + "\n",
		},
		{
			"single document with leading marker",
			toByteSlice(leadmarker + yamlWithoutMarkers1),
			yamlWithoutMarkers1 + "\n",
		},
		{
			"single document with trailing marker",
			toByteSlice(yamlWithoutMarkers1 + marker),
			yamlWithoutMarkers1 + "\n",
		},
		{
			"single document with leading and trailing marker",
			toByteSlice(leadmarker + yamlWithoutMarkers1 + marker),
			yamlWithoutMarkers1 + "\n",
		},
		{
			"single document with both markers and sloppy formatting",
			toByteSlice(el + leadmarker + el + yamlWithoutMarkers1 + el + el + "---"),
			yamlWithoutMarkers1 + "\n",
		},
		{
			"two documents without markers",
			toByteSlice(yamlWithoutMarkers1, yamlWithoutMarkers2),
			twoDocsExpectedOutput,
		},

		{
			"two documents without markers with leading newlines",
			toByteSlice("\n"+yamlWithoutMarkers1, "\n"+yamlWithoutMarkers2),
			twoDocsExpectedOutput,
		},
		{
			"two documents without markers with trailing newlines",
			toByteSlice(yamlWithoutMarkers1+"\n", yamlWithoutMarkers2+"\n"),
			twoDocsExpectedOutput,
		},
		{
			"two documents without markers with leading and trailing newlines",
			toByteSlice("\n"+yamlWithoutMarkers1+"\n", "\n"+yamlWithoutMarkers2+"\n"),
			twoDocsExpectedOutput,
		},
		{
			"three documents without markers",
			toByteSlice(yamlWithoutMarkers1, yamlWithoutMarkers2, yamlWithoutMarkers3),
			threeDocsExpectedOutput,
		},
		{
			"three documents without markers with leading and trailing newlines",
			toByteSlice("\n"+yamlWithoutMarkers1+"\n", "\n"+yamlWithoutMarkers2+"\n", "\n"+yamlWithoutMarkers3+"\n"),
			yamlWithoutMarkers1 + marker + yamlWithoutMarkers2 + marker + yamlWithoutMarkers3 + "\n",
		},
		{
			"first document without marker, second with a leading marker",
			toByteSlice(yamlWithoutMarkers1, leadmarker+yamlWithoutMarkers2),
			twoDocsExpectedOutput,
		},
		{
			"first document without marker, second with a leading and trailing marker",
			toByteSlice(yamlWithoutMarkers1, leadmarker+yamlWithoutMarkers2+marker),
			twoDocsExpectedOutput,
		},
		{
			"first document with a trailing marker, second with a leading and trailing marker",
			toByteSlice(yamlWithoutMarkers1+marker, leadmarker+yamlWithoutMarkers2+marker),
			twoDocsExpectedOutput,
		},
		{
			"two documents with a leading and trailing marker",
			toByteSlice(leadmarker+yamlWithoutMarkers1+marker, leadmarker+yamlWithoutMarkers2+marker),
			twoDocsExpectedOutput,
		},
		{
			"two documents with a leading and trailing marker separated by empty lines from data",
			toByteSlice(
				leadmarker+el+yamlWithoutMarkers1+el+el+marker,
				leadmarker+el+el+yamlWithoutMarkers2+el+marker,
			),
			twoDocsExpectedOutput,
		},
		{
			"two documents with both markers and sloppy formatting",
			toByteSlice(
				el+leadmarker+el+yamlWithoutMarkers1+el+el+"---",
				leadmarker+el+el+yamlWithoutMarkers2+el+"---"+el+el,
			),
			twoDocsExpectedOutput,
		},
		{
			"three documents with a leading and trailing marker separated by empty lines from data",
			toByteSlice(
				leadmarker+el+yamlWithoutMarkers1+el+el+marker,
				leadmarker+el+el+yamlWithoutMarkers2+el+marker,
				leadmarker+el+el+yamlWithoutMarkers3+el+el+marker,
			),
			threeDocsExpectedOutput,
		},
		{
			"three documents with both markers and sloppy formatting",
			toByteSlice(
				el+leadmarker+el+yamlWithoutMarkers1+el+el+marker+el+el,
				el+leadmarker+el+el+yamlWithoutMarkers2+"\n---",
				el+el+leadmarker+el+el+yamlWithoutMarkers3+el+el+"---",
			),
			threeDocsExpectedOutput,
		},
	}

	for _, testCase := range tests {
		tcase := testCase
		t.Run(
			tcase.name, func(t *testing.T) {
				assertions := assert.New(t)
				t.Parallel()
				actual := internal.JoinYAMLDocuments(tcase.yamlDocs)
				assertions.Equal(tcase.expected, actual)
			},
		)
	}
}

func toByteSlice(ls ...string) [][]byte {
	res := make([][]byte, len(ls))
	for i, s := range ls {
		res[i] = []byte(s)
	}
	return res
}

const (
	yamlWithoutMarkers1 = `key11: 111
key12: "value1"`

	yamlWithoutMarkers2 = `key21: 222
key22: "value2"`

	yamlWithoutMarkers3 = `key31: 333
key32: "value3"`

	el         = "  \n"    // empty line with some spaces to simulate poorly formatted file
	marker     = "\n---\n" // standard marker between two documents
	leadmarker = "---\n"   // marker at the beginning of the document

	twoDocsExpectedOutput   = yamlWithoutMarkers1 + marker + yamlWithoutMarkers2 + "\n"
	threeDocsExpectedOutput = yamlWithoutMarkers1 + marker + yamlWithoutMarkers2 + marker + yamlWithoutMarkers3 + "\n"
)
