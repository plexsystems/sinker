package manifest

import "testing"

func TestGetSourceHostFromRepository(t *testing.T) {
	testCases := []struct {
		input              string
		expectedSourceHost string
	}{
		{
			input:              "coreos",
			expectedSourceHost: "quay.io",
		},
		{
			input:              "open-policy-agent",
			expectedSourceHost: "quay.io",
		},
		{
			input:              "kubernetes-ingress-controller",
			expectedSourceHost: "quay.io",
		},
		{
			input:              "twistlock",
			expectedSourceHost: "registry.twistlock.com",
		},
	}

	for _, testCase := range testCases {
		sourceHost := getSourceHostFromRepository(testCase.input)

		if sourceHost != testCase.expectedSourceHost {
			t.Errorf("expected source host to be %s, actual %s", testCase.expectedSourceHost, sourceHost)
		}
	}
}
