#![cfg_attr(not(feature = "std"), no_std)]

use sp_std::prelude::*;
use frame_support::{
	decl_module, decl_storage, decl_event, decl_error,
	weights::Weight,
	traits::Get
};

use sp_io::hashing::keccak_256;
use sp_core::{H160, H256, RuntimeDebug};
use sp_runtime::{
	traits::{Zero, One},
	DigestItem
};

use codec::{Encode, Decode};
use artemis_core::Commitments;

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

/// Custom DigestItem for header digest
#[derive(Encode, Decode, Copy, Clone, PartialEq, RuntimeDebug)]
enum CustomDigestItem {
	Commitment(H256)
}

impl<T> Into<DigestItem<T>> for CustomDigestItem {
    fn into(self) -> DigestItem<T> {
        DigestItem::Other(self.encode())
    }
}

#[derive(Encode, Decode, Clone, PartialEq, RuntimeDebug)]
struct Message {
	address: H160,
	payload: Vec<u8>,
	nonce: u64,
}

pub trait Trait: frame_system::Trait {
	type Event: From<Event> + Into<<Self as frame_system::Trait>::Event>;

	type CommitInterval: Get<Self::BlockNumber>;
}

decl_storage! {
	trait Store for Module<T: Trait> as Commitments {
		/// Nonce
		pub Nonce get(fn nonce): u64;

		/// Messages waiting to be committed
		pub MessageQueue: Vec<Message>;

		/// Committed Messages
		pub Commitment: Vec<Message>;
	}
}

decl_event! {
	pub enum Event {
		Commitment(H256),
	}
}

decl_error! {
	pub enum Error for Module<T: Trait> {}
}

decl_module! {
	pub struct Module<T: Trait> for enum Call where origin: T::Origin {
		type Error = Error<T>;

		fn deposit_event() = default;

		// Generate a message commitment every `T::CommitInterval` blocks.
		//
		// The hash of the commitment is stored as a digest item `CustomDigestItem::Commitment`
		// in the block header. The committed messages are persisted into storage.

		fn on_initialize(now: T::BlockNumber) -> Weight {
			if (now % T::CommitInterval::get()).is_zero() {
				Self::commit()
			} else if (now % T::CommitInterval::get()).is_one() {
				<Self as Store>::Commitment::kill();
				0
			} else {
				0
			}
		}
	}
}

impl<T: Trait> Module<T> {

	// Generate a message commitment
	// TODO: return proper weight
	fn commit() -> Weight {
		let messages: Vec<Message> = <Self as Store>::MessageQueue::take();
		<Self as Store>::Commitment::set(messages.clone());
		let hash: H256 = keccak_256(messages.encode().as_ref()).into();

		let digest_item = CustomDigestItem::Commitment(hash.clone()).into();
		<frame_system::Module<T>>::deposit_log(digest_item);

		Self::deposit_event(Event::Commitment(hash));

		0
	}
}

impl<T: Trait> Commitments for Module<T> {

	// Add a message for eventual inclusion in a commitment
	fn add(address: H160, payload: Vec<u8>) {
		let nonce = <Self as Store>::Nonce::get();
		<Self as Store>::MessageQueue::append(Message { address, payload, nonce });
		<Self as Store>::Nonce::set(nonce + 1);
	}
}
