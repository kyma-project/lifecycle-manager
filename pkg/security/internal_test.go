package security

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_getCertTokenFromXFCCHeader(t *testing.T) {
	const headerPart1 = `By=http://frontend.lyft.com;Hash=468ed33be74eee6556d90c0149c1309e9ba61d6425303443c0748a02dd8de688`
	const headerPart2 = `Subject="/C=US/ST=CA/L=San Francisco/OU=Lyft/CN=Test Client";URI=http://testclient.lyft.com;DNS=lyft.com;DNS=www.lyft.com` //nolint:lll
	const headerCert = `Cert=RnJpIE9jdCAxMyAwODoyMTozMSBBTSBDRVNUIDIwMjMK`

	t.Parallel()
	tests := []struct {
		name           string
		xfccHeader     string
		expectedKeys   int
		expectedResult string
	}{
		{
			name:           "For header without a certificate",
			xfccHeader:     headerPart1 + ";" + headerPart2,
			expectedKeys:   6,
			expectedResult: "", // no "Cert=<value>" key pair
		},
		{
			name:           "For header with the certificate at the beginning",
			xfccHeader:     headerCert + ";" + headerPart1 + ";" + headerPart2,
			expectedKeys:   7,
			expectedResult: "RnJpIE9jdCAxMyAwODoyMTozMSBBTSBDRVNUIDIwMjMK",
		},
		{
			name:           "For header with certificate at the end",
			xfccHeader:     headerPart1 + ";" + headerPart2 + ";" + headerCert,
			expectedKeys:   7,
			expectedResult: "RnJpIE9jdCAxMyAwODoyMTozMSBBTSBDRVNUIDIwMjMK",
		},
		{
			name:           "For header with certificate in the middle",
			xfccHeader:     headerPart1 + ";" + headerCert + ";" + headerPart2,
			expectedKeys:   7,
			expectedResult: "RnJpIE9jdCAxMyAwODoyMTozMSBBTSBDRVNUIDIwMjMK",
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actualResult := getCertTokenFromXFCCHeader(test.xfccHeader)
			require.Equal(t, test.expectedResult, actualResult)
			actualKeys := len(strings.Split(test.xfccHeader, ";"))
			require.Equal(t, test.expectedKeys, actualKeys)
		})
	}
}
