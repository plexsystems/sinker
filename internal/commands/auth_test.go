package commands

import "testing"

func TestGetAuthHostFromRegistryHost(t *testing.T) {
	testCases := []struct {
		input            string
		expectedAuthHost string
	}{
		{
			input:            "",
			expectedAuthHost: "https://index.docker.io/v1/",
		},
		{
			input:            "docker.io",
			expectedAuthHost: "https://index.docker.io/v1/",
		},
		{
			input:            "host.com",
			expectedAuthHost: "host.com",
		},
	}

	for _, testCase := range testCases {
		authHost := getAuthHostFromRegistryHost(testCase.input)

		if authHost != testCase.expectedAuthHost {
			t.Errorf("expected auth host to be %s, actual %s", testCase.expectedAuthHost, authHost)
		}
	}
}
