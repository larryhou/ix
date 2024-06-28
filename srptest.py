#!/usr/bin/env python3
import argparse
import sys

from srptools.context import SRPContext
from srptools.client import SRPClientSession
from srptools.constants import PRIME_3072, PRIME_3072_GEN
from srptools.utils import int_to_bytes
import hashlib, binascii

from pymobiledevice3.remote.tunnel_service import PairingDataComponentTLVBuf,PairingDataComponentType


def main():
    arguments = argparse.ArgumentParser()
    arguments.add_argument('-s', '--salt', required=True, type=str)
    arguments.add_argument('-k', '--private-key', required=True, type=str)
    arguments.add_argument('-p', '--public-key', required=True, type=str)
    options = arguments.parse_args(sys.argv[1:])

    salt = binascii.unhexlify(options.salt)
    client = SRPClientSession(
        SRPContext('Pair-Setup', password='000000', prime=PRIME_3072, generator=PRIME_3072_GEN,
                   hash_func=hashlib.sha512), private=options.private_key)
    ctx = client._context
    print(ctx.pad(ctx._gen).hex())
    print(f'CKEY {client.public}')
    print(f'CKEY CLIENT {int_to_bytes(client._client_public).hex()}')
    r = client.process(
        other_public=options.public_key,
        salt=salt.hex(),
    )

    def get_client_premaster_secret(self, password_hash, server_public, client_private, common_secret):
        """S = (B - (k * g^x)) ^ (a + (u * x)) % N

        :param int server_public:
        :param int password_hash:
        :param int client_private:
        :param int common_secret:
        :rtype: int
        """
        password_verifier = self.get_common_password_verifier(password_hash)
        return pow(
            (server_public - (self._mult * password_verifier)),
            (client_private + (common_secret * password_hash)), self._prime)
    premaster_secret = get_client_premaster_secret(ctx,
                                                   ctx.get_common_password_hash(salt),
                                                   client._server_public,
                                                   client._this_private,
                                                   client._common_secret,
                                                   )
    print(f'premaster_secret {int_to_bytes(premaster_secret).hex()}')
    session_key = ctx.hash(int_to_bytes(premaster_secret), as_bytes=True)

    print(f'KEY/ {client._key.hex()}')
    print(f'PRF/ {client._key_proof.hex()}')

    print(f'PWHS {int_to_bytes(ctx.get_common_password_hash(salt)).hex()}')
    print(f'k {int_to_bytes(ctx._mult).hex()}')
    print(f'_common_secret {int_to_bytes(client._common_secret).hex()}')
    print(f'session_key    {session_key.hex()}')

    client_public = int_to_bytes(client._client_public)
    client_session_key_proof = client._key_proof

    tlv = PairingDataComponentTLVBuf.build([
        {'type': PairingDataComponentType.STATE, 'data': b'\x03'},
        {'type': PairingDataComponentType.PUBLIC_KEY, 'data': client_public[:255]},
        {'type': PairingDataComponentType.PUBLIC_KEY, 'data': client_public[255:]},
        {'type': PairingDataComponentType.PROOF, 'data': client_session_key_proof},
    ])
    print(f'PYTLV {tlv.hex()}')


if __name__ == '__main__': main()
