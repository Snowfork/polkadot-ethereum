require("dotenv").config();

var HDWalletProvider = require("@truffle/hdwallet-provider");

module.exports = {
  networks: {
    // Development
    development: {
      host: "127.0.0.1",
      port: 9545,
      network_id: "*"
    },
    // Integration tests
    e2e_test: {
      host: "127.0.0.1",
      port: 8545,
      network_id: "344"
    },
    ropsten: {
      provider: () => new HDWalletProvider(
        process.env.MNEMONIC,
        "https://ropsten.infura.io/v3/".concat(process.env.INFURA_PROJECT_ID)
      ),
      network_id: 3,
      gas: 8000000,
      gasPrice: 50000000000
    }
  },
  mocha: {
    useColors: true
  },
  compilers: {
    solc: {
      version: "0.8.6",
      optimizer: {
        enabled: true,
        runs: 200
      }
    }
  }
};
