package pathprocessor

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

var (
	ZeroAddress     = common.Address{}
	ZeroBigIntValue = big.NewInt(0)
)

const (
	IncreaseEstimatedGasFactor = 1.1
	SevenDaysInSeconds         = 60 * 60 * 24 * 7

	EthSymbol  = "ETH"
	SntSymbol  = "SNT"
	SttSymbol  = "STT"
	UsdcSymbol = "USDC"
	HopSymbol  = "HOP"

	ProcessorTransferName     = "Transfer"
	ProcessorBridgeHopName    = "Hop"
	ProcessorBridgeCelerName  = "CBridge"
	ProcessorSwapParaswapName = "Paraswap"
	ProcessorERC721Name       = "ERC721Transfer"
	ProcessorERC1155Name      = "ERC1155Transfer"
	ProcessorENSRegisterName  = "ENSRegister"
	ProcessorENSReleaseName   = "ENSRelease"
	ProcessorENSPublicKeyName = "ENSPublicKey"
	ProcessorStickersBuyName  = "StickersBuy"
)
