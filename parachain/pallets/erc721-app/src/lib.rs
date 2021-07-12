//! # ERC721
//!
//! An application that implements bridged ERC721 NFT assets.
//!
//! ## Overview
//!
//! ETH balances are stored in the tightly-coupled [`nft`] runtime module. When an NFT holder burns
//! the token, a `Transfer` event is emitted. An external relayer will listen for this event
//! and relay it to the other chain.
//!
#![cfg_attr(not(feature = "std"), no_std)]

use frame_support::{
	dispatch::DispatchError,
	traits::EnsureOrigin,
	weights::Weight,
};
use frame_support::{pallet_prelude::*, transactional};
use frame_system::{ensure_signed, pallet_prelude::*};

use sp_std::prelude::*;

use sp_runtime::traits::{AtLeast32BitUnsigned, StaticLookup};
use sp_core::H160;

use artemis_core::{ChannelId, OutboundRouter, nft::{Nft, ERC721TokenData}};
use artemis_ethereum::U256;

mod payload;
use payload::OutboundPayload;

mod benchmarking;

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

/// Weight functions needed for this pallet.
pub trait WeightInfo {
	fn burn() -> Weight;
	fn mint() -> Weight;
}

impl WeightInfo for () {
	fn burn() -> Weight { 0 }
	fn mint() -> Weight { 0 }
}

pub use module::*;

#[frame_support::pallet]
pub mod module {
	use super::*;

	/// Weight functions needed for this pallet.
	pub trait WeightInfo {
		fn burn() -> Weight;
		fn mint() -> Weight;
	}

	impl WeightInfo for () {
		fn burn() -> Weight { 0 }
		fn mint() -> Weight { 0 }
	}

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type Event: From<Event<Self>> + IsType<<Self as frame_system::Config>::Event>;

		type OutboundRouter: OutboundRouter<Self::AccountId>;

		type CallOrigin: EnsureOrigin<Self::Origin, Success=H160>;

		/// Weight information for extrinsics in this module.
		type WeightInfo: WeightInfo;

		/// The token ID type, which is the identifier on this parachain and different from the token_id on other chains
		/// such as in an ERC721 contract
		type TokenId: Parameter + Member + AtLeast32BitUnsigned + Default + Copy;

		/// The NFT pallet trait
		type Nft: Nft<Self::AccountId, Self::TokenId, ERC721TokenData>;
	}

	#[pallet::error]
	pub enum Error<T> {
		/// The token is already minted
		TokenAlreadyMinted,
		/// The token does not exist
		TokenNotFound,
	}

	#[pallet::event]
	#[pallet::generate_deposit(pub(crate) fn deposit_event)]
	pub enum Event<T: Config>
	{
		/// Burned event: token_contract, token_id, sender, recipient
		Burned(H160, U256, T::AccountId, H160),
		/// Minted event: token_contract, token_id, sender, recipient
		Minted(H160, U256, H160, T::AccountId),
	}

	/// Address of the peer application on the Ethereum side.
	#[pallet::storage]
	#[pallet::getter(fn address)]
	pub type Address<T: Config> = StorageValue<_, H160, ValueQuery>;

	/// Store ERC721 (contractAddr, contractTokenId) -> Substrate tokenId mapping
	///
	/// Returns `None` if tokenId not set or removed.
	#[pallet::storage]
	#[pallet::getter(fn tokens_by_erc721)]
	pub type TokensByERC721Id<T: Config> = StorageMap<_, Twox64Concat, (H160, U256), T::TokenId>;

	#[pallet::genesis_config]
	pub struct GenesisConfig {
		pub address: H160,
	}

	#[cfg(feature = "std")]
	impl Default for GenesisConfig {
		fn default() -> Self {
			GenesisConfig {
				address: H160::default(),
			}
		}
	}

	#[pallet::genesis_build]
	impl<T: Config> GenesisBuild<T> for GenesisConfig {
		fn build(&self) {
			Address::<T>::try_mutate(|addr| -> Result<H160, DispatchError> {
				*addr = self.address;
				Ok(*addr)
			}).expect("Setting address cannot fail during genesis");
		}
 	}

	#[pallet::pallet]
	pub struct Pallet<T>(PhantomData<T>);

	#[pallet::hooks]
	impl<T: Config> Hooks<T::BlockNumber> for Pallet<T> {}

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		/// Burn an ERC721 token
		#[pallet::weight(T::WeightInfo::burn())]
		#[transactional]
		pub fn burn(origin: OriginFor<T>, channel_id: ChannelId, token: T::TokenId, recipient: H160) -> DispatchResultWithPostInfo {
			let who = ensure_signed(origin)?;

			let token_data = T::Nft::get_token_data(token).ok_or(Error::<T>::TokenNotFound)?;
			let token_contract = token_data.data.token_contract;
			let token_id = token_data.data.token_id;

			T::Nft::burn(&who, token)?;

			// We can assume that the map contains the key, since the token_contract and token_id are extracted from it
			TokensByERC721Id::<T>::remove((token_contract, token_id));

			let message = OutboundPayload {
				token_contract,
				token_id,
				sender: who.clone(),
				recipient,
			};

			T::OutboundRouter::submit(channel_id, &who, Address::<T>::get(), &message.encode())?;
			Self::deposit_event(Event::<T>::Burned(token_contract, token_id, who, recipient));

			Ok(().into())
		}

		#[pallet::weight(T::WeightInfo::mint())]
		#[transactional]
		pub fn mint(origin: OriginFor<T>, sender: H160, recipient: <T::Lookup as StaticLookup>::Source, token_contract: H160, token_id: U256, token_uri: Vec<u8>) -> DispatchResultWithPostInfo {
			let who = T::CallOrigin::ensure_origin(origin)?;
			if who != Address::<T>::get() {
				return Err(DispatchError::BadOrigin.into());
			}
			if TokensByERC721Id::<T>::contains_key((token_contract, token_id)) {
				return Err(Error::<T>::TokenAlreadyMinted.into());
			}

			let recipient = T::Lookup::lookup(recipient)?;
			let token_data = ERC721TokenData{
				token_contract,
				token_id,
			};
			let nft_token_id = T::Nft::mint(&recipient, token_uri, token_data)?;
			TokensByERC721Id::<T>::insert((token_contract, token_id), nft_token_id);

			Self::deposit_event(Event::<T>::Minted(token_contract, token_id, sender, recipient));

			Ok(().into())
		}

	}
}
