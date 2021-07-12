//! ERC721App pallet benchmarking

#![cfg(feature = "runtime-benchmarks")]

use super::*;

use frame_system::RawOrigin;
use frame_support::traits::UnfilteredDispatchable;
use frame_benchmarking::{account, benchmarks, whitelisted_caller, impl_benchmark_test_suite};
use sp_core::H160;
use artemis_core::nft::TokenInfo;

#[allow(unused_imports)]
use crate::Pallet as ERC721App;

benchmarks! {
	// Benchmark `burn` extrinsic under worst case conditions:
	// * `burn` successfully removes token from storage,
	// * the channel executes incentivization logic
	burn {
		let caller: T::AccountId = whitelisted_caller();
		let recipient = H160::repeat_byte(2);
		let token_contract = H160::repeat_byte(4);
		let token_id = U256::one();
		let token_data = ERC721TokenData{
			token_contract,
			token_id,
		};

		let nft_token_id = T::Nft::mint(&caller, "http uri".into(), token_data)?;
		TokensByERC721Id::<T>::insert((token_contract, token_id), nft_token_id);
	}: _(RawOrigin::Signed(caller.clone()), ChannelId::Incentivized, nft_token_id, recipient)
	verify {
		assert_eq!(T::Nft::is_owner(&caller, nft_token_id), false);
		assert_eq!(T::Nft::get_token_data(nft_token_id), None);
	}

	// Benchmark `mint` extrinsic under worst case conditions:
	// * `mint` successfully adds token to storage
	mint {
		let origin = T::CallOrigin::successful_origin();
		if let Ok(caller) = T::CallOrigin::try_origin(origin.clone()) {
			Address::<T>::put(caller);
		} else {
			return Err("Failed to extract caller address from origin");
		}

		let recipient: T::AccountId = account("recipient", 0, 0);
		let recipient_lookup: <T::Lookup as StaticLookup>::Source = T::Lookup::unlookup(recipient.clone());
		let sender = H160::zero();
		let token_contract = H160::repeat_byte(4);
		let token_id = U256::one();

		let call = Call::<T>::mint(sender, recipient_lookup, token_contract, token_id, "http uri".into());

	}: { call.dispatch_bypass_filter(origin)? }
	verify {
		let nft_token_id = TokensByERC721Id::<T>::get((token_contract, token_id));
		assert_eq!(T::Nft::is_owner(&recipient, nft_token_id.unwrap()), true);
		let token_info: TokenInfo<T::AccountId, ERC721TokenData> = TokenInfo{owner: recipient, metadata: "http uri".into(), data: ERC721TokenData{token_contract, token_id}};
		assert_eq!(T::Nft::get_token_data(nft_token_id.unwrap()), Some(token_info));
	}
}

impl_benchmark_test_suite!(
	ERC721App,
	crate::mock::new_tester(),
	crate::mock::Test,
);
