package api

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/status-im/status-go/buildinfo"
	"github.com/status-im/status-go/params"
	"github.com/status-im/status-go/protocol/requests"
)

const (
	mainnetChainID         uint64 = 1
	goerliChainID          uint64 = 5
	sepoliaChainID         uint64 = 11155111
	optimismChainID        uint64 = 10
	optimismGoerliChainID  uint64 = 420
	optimismSepoliaChainID uint64 = 11155420
	arbitrumChainID        uint64 = 42161
	arbitrumGoerliChainID  uint64 = 421613
	arbitrumSepoliaChainID uint64 = 421614
	sntSymbol                     = "SNT"
	sttSymbol                     = "STT"
)

var ganacheTokenAddress = common.HexToAddress("0x8571Ddc46b10d31EF963aF49b6C7799Ea7eff818")

var mainnet = params.Network{
	ChainID:                mainnetChainID,
	ChainName:              "Mainnet",
	DefaultRPCURL:          fmt.Sprintf("https://%s.api.status.im/grove/ethereum/mainnet/", buildinfo.ApiProxyStageName),
	DefaultFallbackURL:     fmt.Sprintf("https://%s.api.status.im/infura/ethereum/mainnet/", buildinfo.ApiProxyStageName),
	RPCURL:                 "https://eth-archival.rpc.grove.city/v1/",
	FallbackURL:            "https://mainnet.infura.io/v3/",
	BlockExplorerURL:       "https://etherscan.io/",
	IconURL:                "network/Network=Ethereum",
	ChainColor:             "#627EEA",
	ShortName:              "eth",
	NativeCurrencyName:     "Ether",
	NativeCurrencySymbol:   "ETH",
	NativeCurrencyDecimals: 18,
	IsTest:                 false,
	Layer:                  1,
	Enabled:                true,
	RelatedChainID:         goerliChainID,
}

var goerli = params.Network{
	ChainID:                goerliChainID,
	ChainName:              "Mainnet",
	RPCURL:                 "https://goerli.infura.io/v3/",
	FallbackURL:            "",
	BlockExplorerURL:       "https://goerli.etherscan.io/",
	IconURL:                "network/Network=Ethereum",
	ChainColor:             "#627EEA",
	ShortName:              "goEth",
	NativeCurrencyName:     "Ether",
	NativeCurrencySymbol:   "ETH",
	NativeCurrencyDecimals: 18,
	IsTest:                 true,
	Layer:                  1,
	Enabled:                true,
	RelatedChainID:         mainnetChainID,
}

var sepolia = params.Network{
	ChainID:                sepoliaChainID,
	ChainName:              "Mainnet",
	DefaultRPCURL:          fmt.Sprintf("https://%s.api.status.im/grove/ethereum/sepolia/", buildinfo.ApiProxyStageName),
	DefaultFallbackURL:     fmt.Sprintf("https://%s.api.status.im/infura/ethereum/sepolia/", buildinfo.ApiProxyStageName),
	RPCURL:                 "https://sepolia-archival.rpc.grove.city/v1/",
	FallbackURL:            "https://sepolia.infura.io/v3/",
	BlockExplorerURL:       "https://sepolia.etherscan.io/",
	IconURL:                "network/Network=Ethereum",
	ChainColor:             "#627EEA",
	ShortName:              "eth",
	NativeCurrencyName:     "Ether",
	NativeCurrencySymbol:   "ETH",
	NativeCurrencyDecimals: 18,
	IsTest:                 true,
	Layer:                  1,
	Enabled:                true,
	RelatedChainID:         mainnetChainID,
}

var optimism = params.Network{
	ChainID:                optimismChainID,
	ChainName:              "Optimism",
	DefaultRPCURL:          fmt.Sprintf("https://%s.api.status.im/grove/optimism/mainnet/", buildinfo.ApiProxyStageName),
	DefaultFallbackURL:     fmt.Sprintf("https://%s.api.status.im/infura/optimism/mainnet/", buildinfo.ApiProxyStageName),
	RPCURL:                 "https://optimism-archival.rpc.grove.city/v1/",
	FallbackURL:            "https://optimism-mainnet.infura.io/v3/",
	BlockExplorerURL:       "https://optimistic.etherscan.io",
	IconURL:                "network/Network=Optimism",
	ChainColor:             "#E90101",
	ShortName:              "oeth",
	NativeCurrencyName:     "Ether",
	NativeCurrencySymbol:   "ETH",
	NativeCurrencyDecimals: 18,
	IsTest:                 false,
	Layer:                  2,
	Enabled:                true,
	RelatedChainID:         optimismGoerliChainID,
}

var optimismGoerli = params.Network{
	ChainID:                optimismGoerliChainID,
	ChainName:              "Optimism",
	RPCURL:                 "https://optimism-goerli.infura.io/v3/",
	FallbackURL:            "",
	BlockExplorerURL:       "https://goerli-optimism.etherscan.io/",
	IconURL:                "network/Network=Optimism",
	ChainColor:             "#E90101",
	ShortName:              "goOpt",
	NativeCurrencyName:     "Ether",
	NativeCurrencySymbol:   "ETH",
	NativeCurrencyDecimals: 18,
	IsTest:                 true,
	Layer:                  2,
	Enabled:                false,
	RelatedChainID:         optimismChainID,
}

var optimismSepolia = params.Network{
	ChainID:                optimismSepoliaChainID,
	ChainName:              "Optimism",
	DefaultRPCURL:          fmt.Sprintf("https://%s.api.status.im/grove/optimism/sepolia/", buildinfo.ApiProxyStageName),
	DefaultFallbackURL:     fmt.Sprintf("https://%s.api.status.im/infura/optimism/sepolia/", buildinfo.ApiProxyStageName),
	RPCURL:                 "https://optimism-sepolia-archival.rpc.grove.city/v1/",
	FallbackURL:            "https://optimism-sepolia.infura.io/v3/",
	BlockExplorerURL:       "https://sepolia-optimism.etherscan.io/",
	IconURL:                "network/Network=Optimism",
	ChainColor:             "#E90101",
	ShortName:              "oeth",
	NativeCurrencyName:     "Ether",
	NativeCurrencySymbol:   "ETH",
	NativeCurrencyDecimals: 18,
	IsTest:                 true,
	Layer:                  2,
	Enabled:                false,
	RelatedChainID:         optimismChainID,
}

var arbitrum = params.Network{
	ChainID:                arbitrumChainID,
	ChainName:              "Arbitrum",
	DefaultRPCURL:          fmt.Sprintf("https://%s.api.status.im/grove/arbitrum/mainnet/", buildinfo.ApiProxyStageName),
	DefaultFallbackURL:     fmt.Sprintf("https://%s.api.status.im/infura/arbitrum/mainnet/", buildinfo.ApiProxyStageName),
	RPCURL:                 "https://arbitrum-one.rpc.grove.city/v1/",
	FallbackURL:            "https://arbitrum-mainnet.infura.io/v3/",
	BlockExplorerURL:       "https://arbiscan.io/",
	IconURL:                "network/Network=Arbitrum",
	ChainColor:             "#51D0F0",
	ShortName:              "arb1",
	NativeCurrencyName:     "Ether",
	NativeCurrencySymbol:   "ETH",
	NativeCurrencyDecimals: 18,
	IsTest:                 false,
	Layer:                  2,
	Enabled:                true,
	RelatedChainID:         arbitrumGoerliChainID,
}

var arbitrumGoerli = params.Network{
	ChainID:                arbitrumGoerliChainID,
	ChainName:              "Arbitrum",
	RPCURL:                 "https://arbitrum-goerli.infura.io/v3/",
	FallbackURL:            "",
	BlockExplorerURL:       "https://goerli.arbiscan.io/",
	IconURL:                "network/Network=Arbitrum",
	ChainColor:             "#51D0F0",
	ShortName:              "goArb",
	NativeCurrencyName:     "Ether",
	NativeCurrencySymbol:   "ETH",
	NativeCurrencyDecimals: 18,
	IsTest:                 true,
	Layer:                  2,
	Enabled:                false,
	RelatedChainID:         arbitrumChainID,
}

var arbitrumSepolia = params.Network{
	ChainID:                arbitrumSepoliaChainID,
	ChainName:              "Arbitrum",
	DefaultRPCURL:          fmt.Sprintf("https://%s.api.status.im/grove/arbitrum/sepolia/", buildinfo.ApiProxyStageName),
	DefaultFallbackURL:     fmt.Sprintf("https://%s.api.status.im/infura/arbitrum/sepolia/", buildinfo.ApiProxyStageName),
	RPCURL:                 "https://arbitrum-sepolia-archival.rpc.grove.city/v1/",
	FallbackURL:            "https://arbitrum-sepolia.infura.io/v3/",
	BlockExplorerURL:       "https://sepolia-explorer.arbitrum.io/",
	IconURL:                "network/Network=Arbitrum",
	ChainColor:             "#51D0F0",
	ShortName:              "arb1",
	NativeCurrencyName:     "Ether",
	NativeCurrencySymbol:   "ETH",
	NativeCurrencyDecimals: 18,
	IsTest:                 true,
	Layer:                  2,
	Enabled:                false,
	RelatedChainID:         arbitrumChainID,
}

var defaultNetworks = []params.Network{
	mainnet,
	goerli,
	sepolia,
	optimism,
	optimismGoerli,
	optimismSepolia,
	arbitrum,
	arbitrumGoerli,
	arbitrumSepolia,
}

var mainnetGanacheTokenOverrides = params.TokenOverride{
	Symbol:  sntSymbol,
	Address: ganacheTokenAddress,
}

var goerliGanacheTokenOverrides = params.TokenOverride{
	Symbol:  sttSymbol,
	Address: ganacheTokenAddress,
}

func setRPCs(networks []params.Network, request *requests.WalletSecretsConfig) []params.Network {

	var networksWithRPC []params.Network

	for _, n := range networks {

		if request.InfuraToken != "" {
			if strings.Contains(n.RPCURL, "infura") {
				n.RPCURL += request.InfuraToken
			}
			if strings.Contains(n.FallbackURL, "infura") {
				n.FallbackURL += request.InfuraToken
			}
		}

		if request.PoktToken != "" {
			if strings.Contains(n.RPCURL, "grove") {
				n.RPCURL += request.PoktToken
			}
			if strings.Contains(n.FallbackURL, "grove") {
				n.FallbackURL += request.PoktToken
			}

		}

		if request.GanacheURL != "" {
			n.RPCURL = request.GanacheURL
			n.FallbackURL = request.GanacheURL
			if n.ChainID == mainnetChainID {
				n.TokenOverrides = []params.TokenOverride{
					mainnetGanacheTokenOverrides,
				}
			} else if n.ChainID == goerliChainID {
				n.TokenOverrides = []params.TokenOverride{
					goerliGanacheTokenOverrides,
				}
			}
		}

		networksWithRPC = append(networksWithRPC, n)
	}

	return networksWithRPC
}

func BuildDefaultNetworks(walletSecretsConfig *requests.WalletSecretsConfig) []params.Network {
	return setRPCs(defaultNetworks, walletSecretsConfig)
}
