
//! Autogenerated weights for incentivized_channel::outbound
//!
//! THIS FILE WAS AUTO-GENERATED USING THE SUBSTRATE BENCHMARK CLI VERSION 3.0.0
//! DATE: 2021-04-15, STEPS: `[50, ]`, REPEAT: 20, LOW RANGE: `[]`, HIGH RANGE: `[]`
//! EXECUTION: Some(Wasm), WASM-EXECUTION: Compiled, CHAIN: Some("/tmp/artemis-benchmark-JNI/spec.json"), DB CACHE: 128

// Executed Command:
// target/release/artemis
// benchmark
// --chain
// /tmp/artemis-benchmark-JNI/spec.json
// --execution
// wasm
// --wasm-execution
// compiled
// --pallet
// incentivized_channel::outbound
// --extrinsic
// *
// --repeat
// 20
// --steps
// 50
// --output
// runtime/snowbridge/src/weights/incentivized_channel_outbound_weights.rs


#![allow(unused_parens)]
#![allow(unused_imports)]

use frame_support::{traits::Get, weights::Weight};
use sp_std::marker::PhantomData;

/// Weight functions for incentivized_channel::outbound.
pub struct WeightInfo<T>(PhantomData<T>);
impl<T: frame_system::Config> incentivized_channel::outbound::WeightInfo for WeightInfo<T> {
	fn on_initialize(m: u32, p: u32, ) -> Weight {
		(0 as Weight)
			// Standard Error: 66_000
			.saturating_add((18_747_000 as Weight).saturating_mul(m as Weight))
			// Standard Error: 7_000
			.saturating_add((896_000 as Weight).saturating_mul(p as Weight))
			.saturating_add(T::DbWeight::get().reads(3 as Weight))
			.saturating_add(T::DbWeight::get().writes(2 as Weight))
	}
	fn on_initialize_non_interval() -> Weight {
		(5_320_000 as Weight)
			.saturating_add(T::DbWeight::get().reads(1 as Weight))
	}
	fn on_initialize_no_messages() -> Weight {
		(8_700_000 as Weight)
			.saturating_add(T::DbWeight::get().reads(2 as Weight))
	}
}
