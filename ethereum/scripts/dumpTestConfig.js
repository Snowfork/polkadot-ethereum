const TOML = require('@iarna/toml');
const fs = require('fs');
const path = require('path');
const BasicInboundChannelProxy = artifacts.require("BasicInboundChannelProxy");

const channelContracts = {
    basic: {
        inbound: artifacts.require("BasicInboundChannel"),
        outbound: artifacts.require("BasicOutboundChannel")
    },
    incentivized: {
        inbound: artifacts.require("IncentivizedInboundChannel"),
        outbound: artifacts.require("IncentivizedOutboundChannel")
    },
}

const bridgeContracts = {
    beefylightclient: artifacts.require("BeefyLightClient"),
}

const channels = {
    basic: {
        inbound: null,
        outbound: null
    },
    incentivized: {
        inbound: null,
        outbound: null
    },
}

const bridge = {
    beefylightclient: null
}

const dump = (tmpDir, channels, bridge) => {
    const config = {
        global: {
            "data-dir": "/tmp/snowbridge-e2e-config",
        },
        ethereum: {
            endpoint: "ws://localhost:8545/",
            startblock: 1,
            "descendants-until-final": 3,
            channels: {
                basic: {
                    inbound: channels.basic.inbound.address,
                    outbound: channels.basic.outbound.address,
                },
                incentivized: {
                    inbound: channels.incentivized.inbound.address,
                    outbound: channels.incentivized.outbound.address,
                },
            },
            beefylightclient: bridge.beefylightclient.address
        },
        parachain: {
            endpoint: "ws://127.0.0.1:11144/"
        },
        relaychain: {
            endpoint: "ws://127.0.0.1:9944/"
        },
        workers: {
            parachaincommitmentrelayer: {
                enabled: true,
                "restart-delay": 30,
            },
            beefyrelayer: {
                enabled: true,
                "restart-delay": 30,
            },
            ethrelayer: {
                enabled: true,
                "restart-delay": 30,
            },
        }
    }
    fs.writeFileSync(path.join(tmpDir, "config.toml"), TOML.stringify(config));
}

module.exports = async (callback) => {
    try {
        let configDir = process.argv[4].toString();
        bridge.beefylightclient = await bridgeContracts.beefylightclient.deployed();
        channels.basic.inbound = await channelContracts.basic.inbound.deployed();
        const network = process.argv[6];
        if (network === 'ropsten' || network === 'e2e_test') {
            channels.basic.inbound = await BasicInboundChannelProxy.deployed();
        }
        channels.basic.outbound = await channelContracts.basic.outbound.deployed();
        channels.incentivized.inbound = await channelContracts.incentivized.inbound.deployed();
        channels.incentivized.outbound = await channelContracts.incentivized.outbound.deployed();
        await dump(configDir, channels, bridge);
    } catch (error) {
        callback(error)
    }
}
