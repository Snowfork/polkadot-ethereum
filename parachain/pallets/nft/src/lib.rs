//! # NFT
//! The NFT (Non-Fungible Token) module provides implementations for handling non fungible assets.
//!
//! - [`nft::Config`](./trait.Config.html)
//! - [`Call`](./enum.Call.html)
//! - [`Module`](./struct.Module.html)
//
//! ## Overview
//!
//! The NFT module is used by the Polkadot-Ethereum bridge to store ERC721 tokens.

//! ## Interface
//!
//! ### Dispatchable Functions
//!
//! - `transfer`: Transfer an NFT (non fungible token) between accounts.
//! - `mint`: Mint an NFT (non fungible token).
//! - `burn`: Burn an NFT (non fungible token).

#![cfg_attr(not(feature = "std"), no_std)]


use frame_support::{ensure, pallet_prelude::*, Parameter};
use sp_runtime::{
	traits::{AtLeast32BitUnsigned, CheckedAdd, MaybeSerializeDeserialize, Member, One},
	DispatchError, DispatchResult,
};
use sp_std::vec::Vec;

use artemis_core::nft::{TokenInfo, Nft};

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

// TODO add weights
/// Weight functions needed for this pallet.
// pub trait WeightInfo {
// 	fn transfer() -> Weight;
// }
//
// impl WeightInfo for () {
// 	fn transfer() -> Weight { 0 }
// }

pub use module::*;

#[frame_support::pallet]
pub mod module {
	use super::*;

	#[pallet::config]
	pub trait Config: frame_system::Config {
		/// The token ID type, which is the identifier on this parachain and different from the token_id on other chains
		/// such as in an ERC721 contract
		type TokenId: Parameter + Member + AtLeast32BitUnsigned + Default + Copy;
		/// The token properties type
		type TokenData: Parameter + Member + MaybeSerializeDeserialize;
	}

	pub type TokenInfoOf<T> = TokenInfo<<T as frame_system::Config>::AccountId, <T as Config>::TokenData>;

	pub type GenesisTokenData<T> = (
		<T as frame_system::Config>::AccountId, // Token owner
		Vec<u8>,                               // Token metadata
		<T as Config>::TokenData,               // Token data
	);

	/// Error for non-fungible-token module.
	#[pallet::error]
	pub enum Error<T> {
		/// No available token ID
		NoAvailableTokenId,
		/// Token not found
		TokenNotFound,
		/// The operator is not the owner of the token and has no permission
		NoPermission,
		/// Arithmetic calculation overflow
		NumOverflow,
	}

	/// Next available token ID.
	#[pallet::storage]
	#[pallet::getter(fn next_token_id)]
	pub type NextTokenId<T: Config> = StorageValue<_, T::TokenId, ValueQuery>;

	/// Store token info.
	///
	/// Returns `None` if token info not set or removed.
	#[pallet::storage]
	#[pallet::getter(fn tokens)]
	pub type Tokens<T: Config> = StorageMap<_, Twox64Concat, T::TokenId, TokenInfoOf<T>>;

	/// Token existence check by owner
	#[pallet::storage]
	pub type TokensByOwner<T: Config> =
		StorageDoubleMap<_, Twox64Concat, T::AccountId, Twox64Concat, T::TokenId, (), ValueQuery>;

	#[pallet::genesis_config]
	pub struct GenesisConfig<T: Config> {
		pub tokens: Vec<GenesisTokenData<T>>,
	}

	#[cfg(feature = "std")]
	impl<T: Config> Default for GenesisConfig<T> {
		fn default() -> Self {
			GenesisConfig { tokens: vec![] }
		}
	}

	#[pallet::genesis_build]
	impl<T: Config> GenesisBuild<T> for GenesisConfig<T> {
		fn build(&self) {
			self.tokens.iter().for_each(|(owner, metadata, data)| {
				Pallet::<T>::mint(&owner.clone(), metadata.clone(), data.clone())
					.expect("Token mint cannot fail during genesis");
			})
		}
	}

	#[pallet::pallet]
	pub struct Pallet<T>(PhantomData<T>);

	#[pallet::hooks]
	impl<T: Config> Hooks<T::BlockNumber> for Pallet<T> {}

	#[pallet::call]
	impl<T: Config> Pallet<T> {}
}

impl<T: Config> Nft<T::AccountId, T::TokenId, T::TokenData> for Pallet<T>
{
	/// Mint NFT (non-fungible token) to `owner`
	fn mint(
		owner: &T::AccountId,
		metadata: Vec<u8>,
		data: T::TokenData,
	) -> Result<T::TokenId, DispatchError> {
		NextTokenId::<T>::try_mutate(|id| -> Result<T::TokenId, DispatchError> {
			let token_id = *id;
			// Should never happen with a sufficiently wide integer, but we check for overflow
			*id = id.checked_add(&One::one()).ok_or(Error::<T>::NumOverflow)?;

			let token_info = TokenInfo {
				metadata,
				owner: owner.clone(),
				data,
			};
			Tokens::<T>::insert(token_id, token_info);
			TokensByOwner::<T>::insert(owner, token_id, ());

			Ok(token_id)
		})
	}

	/// Burn NFT (non-fungible token) from `owner`
	fn burn(owner: &T::AccountId, token: T::TokenId) -> DispatchResult {
		Tokens::<T>::try_mutate_exists(token, |token_info| -> DispatchResult {
			let t = token_info.take().ok_or(Error::<T>::TokenNotFound)?;
			ensure!(t.owner == *owner, Error::<T>::NoPermission);

			TokensByOwner::<T>::remove(owner, token);

			Ok(())
		})
	}

	/// Transfer NFT (non-fungible token) from `from` account to `to` account
	fn transfer(from: &T::AccountId, to: &T::AccountId, token: T::TokenId) -> DispatchResult {
		Tokens::<T>::try_mutate(token, |token_info| -> DispatchResult {
			let mut info = token_info.as_mut().ok_or(Error::<T>::TokenNotFound)?;
			ensure!(info.owner == *from, Error::<T>::NoPermission);
			if from == to {
				// no change needed
				return Ok(());
			}

			info.owner = to.clone();
			TokensByOwner::<T>::remove(from, token);
			TokensByOwner::<T>::insert(to, token, ());

			Ok(())
		})
	}

	fn is_owner(account: &T::AccountId, token: T::TokenId) -> bool {
		TokensByOwner::<T>::contains_key(account, token)
	}

	fn get_token_data(token: T::TokenId) -> Option<TokenInfoOf<T>> {
		Tokens::<T>::get(token)
	}
}
