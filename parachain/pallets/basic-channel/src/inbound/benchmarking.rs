//! BasicInboundChannel pallet benchmarking

#![cfg(feature = "runtime-benchmarks")]

use super::*;

use frame_support::debug;
use frame_system::{RawOrigin, Pallet, self, EventRecord};
use frame_benchmarking::{account, benchmarks, whitelisted_caller, impl_benchmark_test_suite};
use hex_literal::hex;
use sp_core::H160;
use sp_std::convert::TryInto;
use sp_runtime::{ModuleId, traits::AccountIdConversion};

use artemis_core::{ChannelId, Message, MessageId, Proof};
use artemis_ethereum::{Log, Header};

#[allow(unused_imports)]
use crate::inbound::Module as BasicInboundChannel;

fn assert_last_event<T: Config>(system_event: <T as frame_system::Config>::Event) {
	let events = Pallet::<T>::events();
	// compare to the last event record
	let EventRecord { event, .. } = &events[events.len() - 1];
	assert_eq!(event, &system_event);
}

fn make_header_chain(parent: Header, length: u32) -> Vec<Header> {
    let mut chain = vec![parent.clone()];
    (0..length).fold(parent, |ancestor, _| {
        let mut header: Header = Default::default();
        header.parent_hash = ancestor.compute_hash();
        header.number = ancestor.number.saturating_add(1);
        header.difficulty = 1.into();
        chain.push(header.clone());
        header
    });

    chain
}

fn get_dot_module_account_id() -> T::AccountId {
    ModuleId(*b"s/dotapp").get().into_account();
}

benchmarks! {
	// Benchmark `submit` extrinsic under worst case conditions:
	submit {
        let h in 0 .. 20;

        // TODO: somehow set correct source
    
        let caller: T::AccountId = whitelisted_caller();
        let (header, message) = eth_mint_data();
        let envelope: envelope::Envelope = rlp::decode::<Log>(&message.data)
            .map(|log| log.try_into().unwrap())
            .unwrap();
        Nonce::put(envelope.nonce - 1);

        debug::trace!(target: "import_header", "Source: {:?}", envelope.source);

        T::Verifier::initialize_storage(
            make_header_chain(header, h),
            0.into(),
            0, // forces all headers to be finalized
        )?;

	}: _(RawOrigin::Signed(caller.clone()), message)
	verify {
        assert_eq!(envelope.nonce, Nonce::get());

        let message_id = MessageId::new(ChannelId::Basic, envelope.nonce);
        assert_last_event::<T>(T::MessageDispatch::successful_dispatch_event(message_id));
	}

    #[extra]
    submit_2 {
        let h in 0 .. 20;

        let caller: T::AccountId = whitelisted_caller();
        let (header, message) = erc20_mint_data();
        let envelope: envelope::Envelope = rlp::decode::<Log>(&message.data)
            .map(|log| log.try_into().unwrap())
            .unwrap();
        Nonce::put(envelope.nonce - 1);

        debug::trace!(target: "import_header", "Source: {:?}", envelope.source);

        T::Verifier::initialize_storage(
            make_header_chain(header, h),
            0.into(),
            0, // forces all headers to be finalized
        )?;

	}: submit(RawOrigin::Signed(caller.clone()), message)
	verify {
        assert_eq!(envelope.nonce, Nonce::get());

        let message_id = MessageId::new(ChannelId::Basic, envelope.nonce);
        assert_last_event::<T>(T::MessageDispatch::successful_dispatch_event(message_id));
	}

    #[extra]
    submit_3 {
        let h in 0 .. 20;

        debug::trace!(target: "import_header", "Account ID: {:?}", get_dot_module_account_id());
        //let T::MessageDispatch::Call::DOT::

        let caller: T::AccountId = whitelisted_caller();
        let (header, message) = dot_unlock_data();
        let envelope: envelope::Envelope = rlp::decode::<Log>(&message.data)
            .map(|log| log.try_into().unwrap())
            .unwrap();
        Nonce::put(envelope.nonce - 1);

        debug::trace!(target: "import_header", "Source: {:?}", envelope.source);

        T::Verifier::initialize_storage(
            make_header_chain(header, h),
            0.into(),
            0, // forces all headers to be finalized
        )?;

	}: submit(RawOrigin::Signed(caller.clone()), message)
	verify {
        assert_eq!(envelope.nonce, Nonce::get());

        let message_id = MessageId::new(ChannelId::Basic, envelope.nonce);
        assert_last_event::<T>(T::MessageDispatch::successful_dispatch_event(message_id));
	}
}

// ETH mint
// Nonce = 1
// Source = 0x774667629726ec1fabebcec0d9139bd1c8f72a23
fn eth_mint_data() -> (Header, Message) {
    (
        Header {
            parent_hash: hex!("db78099323f15890405671b859d71c93626ebab245e6832f68aa41cbbb4eda17").into(),
            timestamp: 1616191979u64.into(),
            number: 271u64.into(),
            author: hex!("0000000000000000000000000000000000000000").into(),
            transactions_root: hex!("971c977f960468703c703407784de5191163f27630f46aed7fa6e3de52d88006").into(),
            ommers_hash: hex!("1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347").into(),
            extra_data: hex!("").into(),
            state_root: hex!("01f35d940ddbae8af3b5e8d173b9c0f6141d16a39d676d02ecb2211b621615dd").into(),
            receipts_root: hex!("61b27e62e2e96483d54b3a6824a7c53efc8230f99e661ec6e6243bdf8760fef2").into(),
            logs_bloom: (&hex!("00000008000000000000000000000000000000000000400000000000010000000000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000000000008000000000000000000000000000000000000020000000000000000000000000080000000000000004000000020000000000000000000000000000000000000800002000000000000000000000000000000000000000000000000000000000000000")).into(),
            gas_used: 78853u64.into(),
            gas_limit: 6721975u64.into(),
            difficulty: 0u64.into(),
            seal: vec![
                hex!("a00000000000000000000000000000000000000000000000000000000000000000").to_vec(),
                hex!("880000000000000000").to_vec(),
            ],
        },
        Message {
            data: hex!("f90119942ffa5ecdbe006d30397c7636d3e015eee251369fe1a0779b38144a38cfc4351816442048b17fe24ba2b0e0c63446b576e8281160b15bb8e0000000000000000000000000774667629726ec1fabebcec0d9139bd1c8f72a230000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000570c0189b4ab1ef20763630df9743acf155865600daff200d43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d0000c16ff2862300000000000000000000000000000000000000000000000000000000000000000000").to_vec(),
            proof: Proof {
                block_hash: hex!("24317813182a72a8d0ff635c007d0f2bd8609a02add0570781ba20a8d9745c66").into(),
                tx_index: 0,
                data: (
                    vec![hex!("61b27e62e2e96483d54b3a6824a7c53efc8230f99e661ec6e6243bdf8760fef2").to_vec()],
                    vec![hex!("f902cb822080b902c5f902c20183013405b9010000000008000000000000000000000000000000000000400000000000010000000000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000000000008000000000000000000000000000000000000020000000000000000000000000080000000000000004000000020000000000000000000000000000000000000800002000000000000000000000000000000000000000000000000000000000000000f901b7f89994774667629726ec1fabebcec0d9139bd1c8f72a23e1a0caae0f5e72020d428da73a237d1f9bf162e158dda6d4908769b8b60c095b01f4b86000000000000000000000000089b4ab1ef20763630df9743acf155865600daff2d43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d000000000000000000000000000000000000000000000000002386f26fc10000f90119942ffa5ecdbe006d30397c7636d3e015eee251369fe1a0779b38144a38cfc4351816442048b17fe24ba2b0e0c63446b576e8281160b15bb8e0000000000000000000000000774667629726ec1fabebcec0d9139bd1c8f72a230000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000570c0189b4ab1ef20763630df9743acf155865600daff200d43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d0000c16ff2862300000000000000000000000000000000000000000000000000000000000000000000").to_vec()],
                ),
            },
        },
    )
}

// ERC20 mint
// Nonce = 1
// Source = 0x83428c7db9815f482a39a1715684dcf755021997
fn erc20_mint_data() -> (Header, Message) {
    (
        Header {
            parent_hash: hex!("8dcd18f7d4a070bfc636c939f156694518a61fe099d2fe402ddb534afd3a57c9").into(),
            timestamp: 1616463155u64.into(),
            number: 152u64.into(),
            author: hex!("0000000000000000000000000000000000000000").into(),
            transactions_root: hex!("d1d2257ee82cf1a4a623e439b9098c44e7d595534ce186693633d317540e13e4").into(),
            ommers_hash: hex!("1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347").into(),
            extra_data: hex!("").into(),
            state_root: hex!("46d901a61a81b149d01bfaa47f1050b38c40bf8f5aef3446d5fe105d87251f0e").into(),
            receipts_root: hex!("34042a480e6502ad91fa5e8e67852f71279e0742209c82c0676f89c22349bab1").into(),
            logs_bloom: (&hex!("00000008000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000008002000000000000000000000000400000000000000000000000000000000000000000000200010000000000000000010001000000000000008000000000000000004000000100000000000000000800000008000020000000000000001000000000000000200000000000000000000000000000000000002000004000000020000000000000000000008000000800000200c00000010000000000000000000000000000020000000000000000000000000000000")).into(),
            gas_used: 106477u64.into(),
            gas_limit: 6721975u64.into(),
            difficulty: 0u64.into(),
            seal: vec![
                hex!("a00000000000000000000000000000000000000000000000000000000000000000").to_vec(),
                hex!("880000000000000000").to_vec(),
            ],
        },
        Message {
            data: hex!("f9013a942ffa5ecdbe006d30397c7636d3e015eee251369fe1a0779b38144a38cfc4351816442048b17fe24ba2b0e0c63446b576e8281160b15bb9010000000000000000000000000083428c7db9815f482a39a1715684dcf75502199700000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000006b0d01f8f7758fbcefd546eaeff7de24aff666b6228e73be68fc2d8249eb60bfcf0e71d5a0d2f2e292c4ed00d43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27de803000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000").to_vec(),
            proof: Proof {
                block_hash: hex!("c92e389850a4ec477933b23b00296c000e93ef8de88bbd6caa152f13e0733279").into(),
                tx_index: 0,
                data: (
                    vec![hex!("34042a480e6502ad91fa5e8e67852f71279e0742209c82c0676f89c22349bab1").to_vec()],
                    vec![hex!("f90446822080b90440f9043d0183019fedb9010000000008000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000008002000000000000000000000000400000000000000000000000000000000000000000000200010000000000000000010001000000000000008000000000000000004000000100000000000000000800000008000020000000000000001000000000000000200000000000000000000000000000000000002000004000000020000000000000000000008000000800000200c00000010000000000000000000000000000020000000000000000000000000000000f90332f89b94f8f7758fbcefd546eaeff7de24aff666b6228e73f863a0ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3efa0000000000000000000000000be68fc2d8249eb60bfcf0e71d5a0d2f2e292c4eda000000000000000000000000083428c7db9815f482a39a1715684dcf755021997a000000000000000000000000000000000000000000000000000000000000003e8f89b94f8f7758fbcefd546eaeff7de24aff666b6228e73f863a08c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925a0000000000000000000000000be68fc2d8249eb60bfcf0e71d5a0d2f2e292c4eda000000000000000000000000083428c7db9815f482a39a1715684dcf755021997a00000000000000000000000000000000000000000000000000000000000000000f8b99483428c7db9815f482a39a1715684dcf755021997e1a01e7b27577112ed83d53de87b38aee59ab80d8a9ba4acd90aad6cfee917534c79b880000000000000000000000000f8f7758fbcefd546eaeff7de24aff666b6228e73000000000000000000000000be68fc2d8249eb60bfcf0e71d5a0d2f2e292c4edd43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d00000000000000000000000000000000000000000000000000000000000003e8f9013a942ffa5ecdbe006d30397c7636d3e015eee251369fe1a0779b38144a38cfc4351816442048b17fe24ba2b0e0c63446b576e8281160b15bb9010000000000000000000000000083428c7db9815f482a39a1715684dcf75502199700000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000006b0d01f8f7758fbcefd546eaeff7de24aff666b6228e73be68fc2d8249eb60bfcf0e71d5a0d2f2e292c4ed00d43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27de803000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000").to_vec()],
                ),
            },
        },
    )
}

// DOT unlock
// Nonce = 3
// Source = 0xb1185ede04202fe62d38f5db72f71e38ff3e8305
fn dot_unlock_data() -> (Header, Message) {
    (
        Header {
            parent_hash: hex!("105d3ce7ce47e84ddd9d05d4664bfeb63b7fe397fe9abaff73d4cbeff74875e6").into(),
            timestamp: 1616558291u64.into(),
            number: 702u64.into(),
            author: hex!("0000000000000000000000000000000000000000").into(),
            transactions_root: hex!("979b37112184a16bc05f7a6a12eb0b6bd277c1188741315e92aba3517329b091").into(),
            ommers_hash: hex!("1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347").into(),
            extra_data: hex!("").into(),
            state_root: hex!("09b0caa37d5b8ae74786803bf7e3cad6bb718d56cb69c916d330a91c7ff0596b").into(),
            receipts_root: hex!("fe4800a7a5d64ee7a9f2a89d4d9a64530d47842a006c7ce049dbe2fbef851ded").into(),
            logs_bloom: (&hex!("00000008000040000000000000000200000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000400000008000000000000000000008000000000000000000000000000020000000000000000000800000001000400000000000010000000000000000000000000000000000400000000100000000000000000040000008000000000000000000000000000000000000000000000000001000000000200000000000002000004000000020000000000000000000000000000000000000820400000000000000000000000000000000000000000000000000000000000000000")).into(),
            gas_used: 67941u64.into(),
            gas_limit: 6721975u64.into(),
            difficulty: 0u64.into(),
            seal: vec![
                hex!("a00000000000000000000000000000000000000000000000000000000000000000").to_vec(),
                hex!("880000000000000000").to_vec(),
            ],
        },
        Message {
            data: hex!("f90119942ffa5ecdbe006d30397c7636d3e015eee251369fe1a0779b38144a38cfc4351816442048b17fe24ba2b0e0c63446b576e8281160b15bb8e0000000000000000000000000b1185ede04202fe62d38f5db72f71e38ff3e83050000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000570e0189b4ab1ef20763630df9743acf155865600daff200d43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d000064a7b3b6e00d000000000000000000000000000000000000000000000000000000000000000000").to_vec(),
            proof: Proof {
                block_hash: hex!("de7dc91925b30179f092a2f7dbf4c187ae6d9aa77ad8c3a6ac90094b7616025a").into(),
                tx_index: 0,
                data: (
                    vec![hex!("fe4800a7a5d64ee7a9f2a89d4d9a64530d47842a006c7ce049dbe2fbef851ded").to_vec()],
                    vec![hex!("f9040c822080b90406f904030183010965b9010000000008000040000000000000000200000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000400000008000000000000000000008000000000000000000000000000020000000000000000000800000001000400000000000010000000000000000000000000000000000400000000100000000000000000040000008000000000000000000000000000000000000000000000000001000000000200000000000002000004000000020000000000000000000000000000000000000820400000000000000000000000000000000000000000000000000000000000000000f902f8f9013c94672a95c8928c8450b594186cf7954ec269626a2df863a0a78a9be3a7b862d26933ad85fb11d80ef66b8f972d7cbba06621d583943a4098a0000000000000000000000000b1185ede04202fe62d38f5db72f71e38ff3e8305a000000000000000000000000089b4ab1ef20763630df9743acf155865600daff2b8c00000000000000000000000000000000000000000000000000de0b6b3a7640000000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000020d43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d0000000000000000000000000000000000000000000000000000000000000000f89b94672a95c8928c8450b594186cf7954ec269626a2df863a0ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3efa000000000000000000000000089b4ab1ef20763630df9743acf155865600daff2a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000de0b6b3a7640000f90119942ffa5ecdbe006d30397c7636d3e015eee251369fe1a0779b38144a38cfc4351816442048b17fe24ba2b0e0c63446b576e8281160b15bb8e0000000000000000000000000b1185ede04202fe62d38f5db72f71e38ff3e83050000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000570e0189b4ab1ef20763630df9743acf155865600daff200d43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d000064a7b3b6e00d000000000000000000000000000000000000000000000000000000000000000000").to_vec()],
                ),
            },
        },
    )
}

impl_benchmark_test_suite!(
	BasicInboundChannel,
	crate::mock::new_tester(),
	crate::mock::Test,
);
