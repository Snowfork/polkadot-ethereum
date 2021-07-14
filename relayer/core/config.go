package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/snowfork/polkadot-ethereum/relayer/chain/ethereum"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/parachain"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/relaychain"
	"github.com/snowfork/polkadot-ethereum/relayer/workers"
	"github.com/spf13/viper"
)

type WorkerConfig struct {
	ParachainCommitmentRelayer workers.WorkerConfig `mapstructure:"parachaincommitmentrelayer"`
	BeefyRelayer               workers.WorkerConfig `mapstructure:"beefyrelayer"`
	EthRelayer                 workers.WorkerConfig `mapstructure:"ethrelayer"`
}

type GlobalConfig struct {
	DataDir string `mapstructure:"data-dir"`
}

type Config struct {
	Global               GlobalConfig      `mapstructure:"global"`
	Eth                  ethereum.Config   `mapstructure:"ethereum"`
	Parachain            parachain.Config  `mapstructure:"parachain"`
	Relaychain           relaychain.Config `mapstructure:"relaychain"`
	Workers              WorkerConfig      `mapstructure:"workers"`
}

func LoadConfig() (*Config, error) {
	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	// Load secrets from environment variables
	var value string
	var ok bool

	// Ethereum configuration
	value, ok = os.LookupEnv("SNOWBRIDGE_BEEFY_KEY")
	if !ok {
		return nil, fmt.Errorf("environment variable not set: SNOWBRIDGE_BEEFY_KEY")
	}
	config.Eth.BeefyPrivateKey = strings.TrimPrefix(value, "0x")

	value, ok = os.LookupEnv("SNOWBRIDGE_MESSAGE_KEY")
	if !ok {
		return nil, fmt.Errorf("environment variable not set: SNOWBRIDGE_MESSAGE_KEY")
	}
	config.Eth.MessagePrivateKey = strings.TrimPrefix(value, "0x")

	// Parachain configuration
	value, ok = os.LookupEnv("SNOWBRIDGE_PARACHAIN_KEY")
	if !ok {
		return nil, fmt.Errorf("environment variable not set: SNOWBRIDGE_PARACHAIN_KEY")
	}
	config.Parachain.PrivateKey = value

	// Relaychain configuration
	value, ok = os.LookupEnv("SNOWBRIDGE_RELAYCHAIN_KEY")
	if !ok {
		return nil, fmt.Errorf("environment variable not set: SNOWBRIDGE_RELAYCHAIN_KEY")
	}
	config.Relaychain.PrivateKey = value

	return &config, nil
}
