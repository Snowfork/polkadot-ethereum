require("dotenv").config();
const fs = require('fs');

contractNames = [
  "ETHApp",
  "ERC20App",
  "DOTApp",
  "BasicInboundChannel",
  "BasicOutboundChannel",
  "IncentivizedInboundChannel",
  "IncentivizedOutboundChannel"
]

module.exports = function (deployer, network, accounts) {
  deployer.then(async () => {

    addresses = {}

    for (const name of contractNames) {
      contract = await artifacts.require(name)
      instance = await contract.deployed()

      addresses[name] = instance.address
    }

    console.log(JSON.stringify(addresses, null, 2))

    console.log("Dumped contract addresses at /tmp/addresses.json")
    fs.writeFileSync("/tmp/addresses.json", JSON.stringify(addresses, null, 2))
  });
}
