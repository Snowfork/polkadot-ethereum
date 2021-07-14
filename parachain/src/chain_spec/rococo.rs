use cumulus_primitives_core::ParaId;
use hex_literal::hex;
use rococo_runtime::{
	GenesisConfig, WASM_BINARY, Signature, AccountId, AuraId
};
use sc_service::{ChainType, Properties};
use sp_core::{sr25519, Pair, Public, U256};
use sp_runtime::{Perbill, traits::{IdentifyAccount, Verify}};

use super::{get_from_seed, Extensions};

/// Specialized `ChainSpec`. This is a specialization of the general Substrate ChainSpec type.
pub type ChainSpec = sc_service::GenericChainSpec<GenesisConfig, Extensions>;

use artemis_core::AssetId;

type AccountPublic = <Signature as Verify>::Signer;

/// Helper function to generate an account ID from seed
pub fn get_account_id_from_seed<TPublic: Public>(seed: &str) -> AccountId
where
	AccountPublic: From<<TPublic::Pair as Pair>::Public>,
{
	AccountPublic::from(get_from_seed::<TPublic>(seed)).into_account()
}

pub fn get_chain_spec(para_id: ParaId) -> ChainSpec {
	let mut props = Properties::new();
	props.insert("tokenSymbol".into(), "ROC".into());
	props.insert("tokenDecimals".into(), 12.into());

	ChainSpec::from_genesis(
		"Artemis Local Testnet",
		"local_testnet",
		ChainType::Local,
		move || {
			testnet_genesis(
				vec![
					get_from_seed::<AuraId>("Alice"),
					get_from_seed::<AuraId>("Bob"),
				],
				vec![
					get_account_id_from_seed::<sr25519::Public>("Alice"),
					get_account_id_from_seed::<sr25519::Public>("Bob"),
					get_account_id_from_seed::<sr25519::Public>("Charlie"),
					get_account_id_from_seed::<sr25519::Public>("Dave"),
					get_account_id_from_seed::<sr25519::Public>("Eve"),
					get_account_id_from_seed::<sr25519::Public>("Ferdie"),
					get_account_id_from_seed::<sr25519::Public>("Relay"),
					get_account_id_from_seed::<sr25519::Public>("Alice//stash"),
					get_account_id_from_seed::<sr25519::Public>("Bob//stash"),
					get_account_id_from_seed::<sr25519::Public>("Charlie//stash"),
					get_account_id_from_seed::<sr25519::Public>("Dave//stash"),
					get_account_id_from_seed::<sr25519::Public>("Eve//stash"),
					get_account_id_from_seed::<sr25519::Public>("Ferdie//stash"),
				],
				para_id
			)
		},
		vec![],
		None,
		None,
		Some(props),
		Extensions {
			relay_chain: "local_testnet".into(),
			para_id: para_id.into(),
		},
	)
}

/// Configure initial storage state for FRAME modules.
fn testnet_genesis(
	initial_authorities: Vec<AuraId>,
	endowed_accounts: Vec<AccountId>,
	para_id: ParaId
) -> GenesisConfig {
	GenesisConfig {
		frame_system: rococo_runtime::SystemConfig {
			// Add Wasm runtime to storage.
			code: WASM_BINARY
				.expect("WASM binary was not build, please build it!")
				.to_vec(),
			changes_trie_config: Default::default(),
		},
		pallet_balances: rococo_runtime::BalancesConfig {
			// Configure endowed accounts with initial balance of 1 << 60.
			balances: endowed_accounts.iter().cloned().map(|k|(k, 1 << 60)).collect(),
		},
		pallet_sudo: rococo_runtime::SudoConfig { key: get_account_id_from_seed::<sr25519::Public>("Alice") },
		pallet_collective_Instance1: Default::default(),
		pallet_membership_Instance1: rococo_runtime::LocalCouncilMembershipConfig {
			members: vec![
				get_account_id_from_seed::<sr25519::Public>("Alice"),
				get_account_id_from_seed::<sr25519::Public>("Bob"),
				get_account_id_from_seed::<sr25519::Public>("Charlie"),
				get_account_id_from_seed::<sr25519::Public>("Dave"),
			],
			phantom: Default::default()
		},
		basic_channel_inbound: rococo_runtime::BasicInboundChannelConfig {
			source_channel: hex!["B8EA8cB425d85536b158d661da1ef0895Bb92F1D"].into(),
		},
		basic_channel_outbound: rococo_runtime::BasicOutboundChannelConfig {
			principal: get_account_id_from_seed::<sr25519::Public>("Alice"),
			interval: 1,
		},
		incentivized_channel_inbound: rococo_runtime::IncentivizedInboundChannelConfig {
			source_channel: hex!["3f0839385DB9cBEa8E73AdA6fa0CFe07E321F61d"].into(),
			reward_fraction: Perbill::from_percent(80)
		},
		incentivized_channel_outbound: rococo_runtime::IncentivizedOutboundChannelConfig {
			fee: U256::from_str_radix("10000000000000000", 10).unwrap(), // 0.01 SnowEther
			interval: 1,
		},
		assets: rococo_runtime::AssetsConfig {
			balances: vec![
				(
					AssetId::ETH,
					get_account_id_from_seed::<sr25519::Public>("Ferdie"),
					U256::from_str_radix("1000000000000000000", 10).unwrap()
				)
			]
		},
		nft: rococo_runtime::NFTConfig {
			tokens: vec![]
		},
		verifier_lightclient: rococo_runtime::VerifierLightclientConfig {
			initial_header: Default::default(),
			initial_difficulty: Default::default()
		},
		eth_app: rococo_runtime::ETHConfig {
			address: hex!["440eDFFA1352B13227e8eE646f3Ea37456deC701"].into()
		},
		erc20_app: rococo_runtime::ERC20Config {
			address: hex!["54D6643762E46036b3448659791adAf554225541"].into()
		},
		dot_app: rococo_runtime::DOTConfig {
			address: hex!["dAF13FA1997b9649b2bCC553732c67887A68022C"].into(),
			phantom: Default::default(),
		},
		erc721_app: rococo_runtime::ERC721Config {
			address: hex!["433488cec14C4478e5ff18DDC7E7384Fc416f148"].into(),
		},
		parachain_info: rococo_runtime::ParachainInfoConfig { parachain_id: para_id },
		pallet_aura: rococo_runtime::AuraConfig {
			authorities: initial_authorities,
		},
		cumulus_pallet_aura_ext: Default::default(),
	}
}
