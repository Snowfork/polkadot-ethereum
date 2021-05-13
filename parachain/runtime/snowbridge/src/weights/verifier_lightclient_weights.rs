
//! Autogenerated weights for verifier_lightclient
//!
//! THIS FILE WAS AUTO-GENERATED USING THE SUBSTRATE BENCHMARK CLI VERSION 3.0.0
//! DATE: 2021-05-08, STEPS: `[50, ]`, REPEAT: 20, LOW RANGE: `[]`, HIGH RANGE: `[]`
//! EXECUTION: Some(Wasm), WASM-EXECUTION: Compiled, CHAIN: Some("/tmp/artemis-benchmark-tce/spec.json"), DB CACHE: 128

// Executed Command:
// target/release/artemis
// benchmark
// --chain
// /tmp/artemis-benchmark-tce/spec.json
// --execution
// wasm
// --wasm-execution
// compiled
// --pallet
// verifier_lightclient
// --extrinsic
// *
// --repeat
// 20
// --steps
// 50
// --output
// runtime/snowbridge/src/weights/verifier_lightclient_weights.rs


#![allow(unused_parens)]
#![allow(unused_imports)]

use frame_support::{traits::Get, weights::Weight};
use sp_std::marker::PhantomData;

/// Weight functions for verifier_lightclient.
pub struct WeightInfo<T>(PhantomData<T>);
impl<T: frame_system::Config> verifier_lightclient::WeightInfo for WeightInfo<T> {
	fn import_header() -> Weight {
		(1_433_779_000 as Weight)
			.saturating_add(T::DbWeight::get().reads(17 as Weight))
			.saturating_add(T::DbWeight::get().writes(22 as Weight))
	}
	fn import_header_not_new_finalized_with_max_prune() -> Weight {
		(1_398_977_000 as Weight)
			.saturating_add(T::DbWeight::get().reads(16 as Weight))
			.saturating_add(T::DbWeight::get().writes(20 as Weight))
	}
	fn import_header_new_finalized_with_single_prune() -> Weight {
		(1_355_714_000 as Weight)
			.saturating_add(T::DbWeight::get().reads(10 as Weight))
			.saturating_add(T::DbWeight::get().writes(8 as Weight))
	}
	fn import_header_not_new_finalized_with_single_prune() -> Weight {
		(1_321_282_000 as Weight)
			.saturating_add(T::DbWeight::get().reads(9 as Weight))
			.saturating_add(T::DbWeight::get().writes(6 as Weight))
	}
}
