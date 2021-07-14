package ethereum

type Config struct {
	Endpoint                       string         `mapstructure:"endpoint"`
	BeefyPrivateKey                string         `mapstructure:"beefy-private-key"`
	MessagePrivateKey			   string         `mapstructure:"message-private-key"`
	DescendantsUntilFinal          byte           `mapstructure:"descendants-until-final"`
	Channels                       ChannelsConfig `mapstructure:"channels"`
	BeefyLightClient               string         `mapstructure:"beefylightclient"`
	StartBlock                     uint64         `mapstructure:"startblock"`
}

type ChannelsConfig struct {
	Basic        ChannelConfig `mapstructure:"basic"`
	Incentivized ChannelConfig `mapstructure:"incentivized"`
}

type ChannelConfig struct {
	Inbound  string `mapstructure:"inbound"`
	Outbound string `mapstructure:"outbound"`
}
