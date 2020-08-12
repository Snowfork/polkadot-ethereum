// Mock runtime

use crate::{Module, Trait};
use sp_core::H256;
use frame_support::{impl_outer_origin, impl_outer_event, parameter_types, weights::Weight, dispatch::DispatchResult};
use sp_runtime::{
	traits::{BlakeTwo256, IdentityLookup, IdentifyAccount, Verify}, testing::Header, Perbill, MultiSignature
};
use frame_system as system;

use artemis_generic_asset as generic_asset;
use pallet_bridge as bridge;

use artemis_core::{Broker, AppID, Message};

impl_outer_origin! {
	pub enum Origin for MockRuntime {}
}

mod test_events {
    pub use crate::Event;
}

impl_outer_event! {
    pub enum MockEvent for MockRuntime {
		system<T>,
		bridge<T>,
		generic_asset<T>,
        test_events,
    }
}

pub type Signature = MultiSignature;

pub type AccountId = <<Signature as Verify>::Signer as IdentifyAccount>::AccountId;

#[derive(Clone, Eq, PartialEq)]
pub struct MockRuntime;

parameter_types! {
	pub const BlockHashCount: u64 = 250;
	pub const MaximumBlockWeight: Weight = 1024;
	pub const MaximumBlockLength: u32 = 2 * 1024;
	pub const AvailableBlockRatio: Perbill = Perbill::from_percent(75);
}

impl system::Trait for MockRuntime {
	type BaseCallFilter = ();
	type Origin = Origin;
	type Call = ();
	type Index = u64;
	type BlockNumber = u64;
	type Hash = H256;
	type Hashing = BlakeTwo256;
	type AccountId = AccountId;
	type Lookup = IdentityLookup<Self::AccountId>;
	type Header = Header;
	type Event = MockEvent;
	type BlockHashCount = BlockHashCount;
	type MaximumBlockWeight = MaximumBlockWeight;
	type DbWeight = ();
	type BlockExecutionWeight = ();
	type ExtrinsicBaseWeight = ();
	type MaximumExtrinsicWeight = MaximumBlockWeight;
	type MaximumBlockLength = MaximumBlockLength;
	type AvailableBlockRatio = AvailableBlockRatio;
	type Version = ();
	type ModuleToIndex = ();
	type AccountData = ();
	type OnNewAccount = ();
	type OnKilledAccount = ();
}

impl generic_asset::Trait for MockRuntime {
	type Event = MockEvent;
}


pub struct MockBroker;
impl Broker for MockBroker {
	fn submit(_app_id: AppID, _message: Message) -> DispatchResult {
		Ok(())
	}
}

impl bridge::Trait for MockRuntime {
	type Event = MockEvent;
	type Broker = MockBroker;
}

impl Trait for MockRuntime {
	type Event = MockEvent;
	type Bridge = bridge::Module<MockRuntime>;
}

pub type System = system::Module<MockRuntime>;
pub type GenericAsset = generic_asset::Module<MockRuntime>;
pub type ETH = Module<MockRuntime>;

pub fn new_tester() -> sp_io::TestExternalities {
	let storage = system::GenesisConfig::default().build_storage::<MockRuntime>().unwrap();
	let mut ext: sp_io::TestExternalities = storage.into();
	ext.execute_with(|| System::set_block_number(1));
	ext
}

