let { ApiPromise, WsProvider } = require('@polkadot/api');
const { default: BigNumber } = require('bignumber.js');

class SubClient {

    constructor(endpoint) {
        this.endpoint = endpoint
        this.api = null
    }

    async connect() {
        const provider = new WsProvider('ws://127.0.0.1:9944');
        this.api = await ApiPromise.create({
            provider,
            types: {
                Address: 'AccountId',
                LookupSource: 'AccountId',
                AppId: '[u8; 20]',
                Message: {
                    payload: 'Vec<u8>',
                    verification: 'VerificationInput'
                },
                VerificationInput: {
                    _enum: {
                        Basic: 'VerificationBasic',
                        None: null
                    }
                },
                VerificationBasic: {
                    blockNumber: 'u64',
                    eventIndex: 'u32'
                },
                TokenId: 'H160',
                BridgedAssetId: 'H160',
                AssetAccountData: {
                    free: 'U256'
                }
            }
        })
    }

    async queryAccountBalance(accountId, assetId) {
        let accountData = await this.api.query.asset.account(assetId, accountId);
        if (accountData && accountData.free) {
            return BigNumber(accountData.free.toBigInt())
        }
        return null
    }

}

module.exports.SubClient = SubClient;

