package ens

import (
	"testing"

	"github.com/coredns/caddy"
)

func TestENSParse(t *testing.T) {
	tests := []struct {
		key                string
		inputFileRules     string
		err                string
		connection         string
		ethlinknameservers []string
		ipfsgatewayas      []string
		ipfsgatewayaaaas   []string
	}{
		{ // 0
			".",
			`ens {
			}`,
			"Testfile:2 - Error during parsing: no connection",
			"",
			nil,
			nil,
			nil,
		},
		{ // 1
			".",
			`ens {
			   connection
			}`,
			"Testfile:2 - Error during parsing: invalid connection; no value",
			"",
			nil,
			nil,
			nil,
		},
		{ // 2
			".eth.link",
			`ens {
			  connection /home/test/.ethereum/geth.ipc
			  ethlinknameservers ns1.ethdns.xyz
			}`,
			"",
			"/home/test/.ethereum/geth.ipc",
			[]string{"ns1.ethdns.xyz."},
			nil,
			nil,
		},
		{ // 3
			".",
			`ens {
			  connection http://localhost:8545/
			  ethlinknameservers ns1.ethdns.xyz ns2.ethdns.xyz
			}`,
			"",
			"http://localhost:8545/",
			[]string{"ns1.ethdns.xyz.", "ns2.ethdns.xyz."},
			nil,
			nil,
		},
		{ // 4
			".",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewaya
			}`,
			"Testfile:3 - Error during parsing: invalid IPFS gateway A; no value",
			"",
			nil,
			nil,
			nil,
		},
		{ // 5
			".",
			`ens {
			  connection http://localhost:8545/
			  ethlinknameservers ns1.ethdns.xyz ns2.ethdns.xyz
			  ipfsgatewaya 193.62.81.1
			}`,
			"",
			"",
			nil,
			[]string{"193.62.81.1"},
			nil,
		},
		{ // 6
			".",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewayaaaa
			}`,
			"Testfile:3 - Error during parsing: invalid IPFS gateway AAAA; no value",
			"",
			nil,
			nil,
			nil,
		},
		{ // 7
			".",
			`ens {
			  connection http://localhost:8545/
			  ethlinknameservers ns1.ethdns.xyz ns2.ethdns.xyz
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7
			}`,
			"",
			"",
			nil,
			nil,
			[]string{"fe80::b8fb:325d:fb5a:40e7"},
		},
		{ // 8
			"tls://.:8053",
			`ens {
			  connection http://localhost:8545/
			  ethlinknameservers ns1.ethdns.xyz ns2.ethdns.xyz
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7
			}`,
			"",
			"",
			nil,
			nil,
			[]string{"fe80::b8fb:325d:fb5a:40e7"},
		},
		{ // 9
			".:8053",
			`ens {
			  connection http://localhost:8545/ bad
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7
			}`,
			"Testfile:2 - Error during parsing: invalid connection; multiple values",
			"",
			nil,
			nil,
			nil,
		},
		{ // 10
			".:8053",
			`ens {
			  connection http://localhost:8545/
			  ethlinknameservers ns1.ethdns.xyz ns2.ethdns.xyz
			  ipfsgatewaya 193.62.81.1 193.62.81.2
			}`,
			"",
			"",
			nil,
			[]string{"193.62.81.1", "193.62.81.2"},
			nil,
		},
		{ // 11
			".:8053",
			`ens {
			  connection http://localhost:8545/
			  ethlinknameservers ns1.ethdns.xyz ns2.ethdns.xyz
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7 fe80::b8fb:325d:fb5a:40e8
			}`,
			"",
			"",
			nil,
			nil,
			[]string{"fe80::b8fb:325d:fb5a:40e7", "fe80::b8fb:325d:fb5a:40e8"},
		},
		{ // 12
			".:8053",
			`ens {
			  connection http://localhost:8545/
			  ipfsgatewayaaaa fe80::b8fb:325d:fb5a:40e7 fe80::b8fb:325d:fb5a:40e8
			  bad
			}`,
			"Testfile:4 - Error during parsing: unknown value bad",
			"",
			nil,
			nil,
			nil,
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("ens", test.inputFileRules)
		c.Key = test.key
		connection, ethlinknameservers, ipfsgatewayas, ipfsgatewayaaaas, err := ensParse(c)

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
				if test.ethlinknameservers != nil {
					if len(ethlinknameservers) != len(test.ethlinknameservers) {
						t.Fatalf("Test %d ethlinknameservers expected %v entries, got %v", i, len(test.ethlinknameservers), len(ethlinknameservers))
					}
					for j := range test.ethlinknameservers {
						if ethlinknameservers[j] != test.ethlinknameservers[j] {
							t.Fatalf("Test %d ethlinknameservers expected %v, got %v", i, test.ethlinknameservers[j], ethlinknameservers[j])
						}
					}
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
