package ens

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestENSParse(t *testing.T) {
	tests := []struct {
		key              string
		inputFileRules   string
		err              string
		connection       string
		ipfsgatewayas    []string
		ipfsgatewayaaaas []string
	}{
		{ // 0
			".eth.link",
			`ens {
			}`,
			"Testfile:2 - Error during parsing: no connection",
			"",
			nil,
			nil,
		},
		{ // 1
			".eth.link",
			`ens {
			   connection
			}`,
			"Testfile:2 - Error during parsing: invalid connection; no value",
			"",
			nil,
			nil,
		},
		{ // 2
			".eth.link",
			`ens {
			  connection /home/test/.ethereum/geth.ipc
			}`,
			"",
			"/home/test/.ethereum/geth.ipc",
			nil,
			nil,
		},
		{ // 3
			".eth.link",
			`ens {
			  connection http://localhost:8545/
			}`,
			"",
			"http://localhost:8545/",
			nil,
			nil,
		},
		{ // 4
			".eth.link",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewaya
			}`,
			"Testfile:3 - Error during parsing: invalid IPFS gateway A; no value",
			"",
			nil,
			nil,
		},
		{ // 5
			".eth.link",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewaya 193.62.81.1
			}`,
			"",
			"",
			[]string{"193.62.81.1"},
			nil,
		},
		{ // 6
			".eth.link",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewayaaaa
			}`,
			"Testfile:3 - Error during parsing: invalid IPFS gateway AAAA; no value",
			"",
			nil,
			nil,
		},
		{ // 7
			".eth.link",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7
			}`,
			"",
			"",
			nil,
			[]string{"fe80::b8fb:325d:fb5a:40e7"},
		},
		{ // 8
			"tls://eth.link:8053",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7
			}`,
			"",
			"",
			nil,
			[]string{"fe80::b8fb:325d:fb5a:40e7"},
		},
		{ // 9
			"eth.link:8053",
			`ens {
			  connection http://localhost:8545/ bad
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7
			}`,
			"Testfile:2 - Error during parsing: invalid connection; multiple values",
			"",
			nil,
			nil,
		},
		{ // 10
			"eth.link:8053",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewaya 193.62.81.1 193.62.81.2
			}`,
			"",
			"",
			[]string{"193.62.81.1", "193.62.81.2"},
			nil,
		},
		{ // 11
			"eth.link:8053",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7 fe80::b8fb:325d:fb5a:40e8
			}`,
			"",
			"",
			nil,
			[]string{"fe80::b8fb:325d:fb5a:40e7", "fe80::b8fb:325d:fb5a:40e8"},
		},
		{ // 12
			"eth.link:8053",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7 fe80::b8fb:325d:fb5a:40e8
			  bad
			}`,
			"Testfile:4 - Error during parsing: unknown value bad",
			"",
			nil,
			nil,
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("ens", test.inputFileRules)
		c.Key = test.key
		connection, _, ipfsgatewayas, ipfsgatewayaaaas, err := ensParse(c)

		if test.err != "" {
			if err == nil {
				t.Fatalf("Failed to obtain expected error at test %d", i)
			}
			if err.Error() != test.err {
				t.Fatalf("Unexpected error \"%s\" at test %d", err.Error(), i)
			}
		} else {
			if err != nil {
				t.Fatalf("Unexpected error \"%s\" at test %d", err.Error(), i)
			} else {
				if test.connection != "" && connection != test.connection {
					t.Fatalf("Test %d connection expected %v, got %v", i, test.connection, connection)
				}
				if test.ipfsgatewayas != nil {
					if len(ipfsgatewayas) != len(test.ipfsgatewayas) {
						t.Fatalf("Test %d ipfsgatewayas expected %v entries, got %v", i, len(test.ipfsgatewayas), len(ipfsgatewayas))
					}
					for j := range test.ipfsgatewayas {
						if ipfsgatewayas[j] != test.ipfsgatewayas[j] {
							t.Fatalf("Test %d ipfsgatewayas expected %v, got %v", i, test.ipfsgatewayas[j], ipfsgatewayas[j])
						}
					}
				}
				if test.ipfsgatewayaaaas != nil {
					if len(ipfsgatewayaaaas) != len(test.ipfsgatewayaaaas) {
						t.Fatalf("Test %d ipfsgatewayaaaas expected %v entries, got %v", i, len(test.ipfsgatewayaaaas), len(ipfsgatewayaaaas))
					}
					for j := range test.ipfsgatewayaaaas {
						if ipfsgatewayaaaas[j] != test.ipfsgatewayaaaas[j] {
							t.Fatalf("Test %d ipfsgatewayaaaas expected %v, got %v", i, test.ipfsgatewayaaaas[j], ipfsgatewayaaaas[j])
						}
					}
				}
			}
		}
	}
}
