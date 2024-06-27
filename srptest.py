#!/usr/bin/env python3

from srptools.context import SRPContext
from srptools.client import SRPClientSession
from srptools.constants import PRIME_3072,PRIME_3072_GEN
from srptools.utils import int_to_bytes
import hashlib, binascii

def main():
    private = '781b8dd23a15c2c67bf893ee335ec593b22117fa3251f1380f67d2df78b16a17e0cbb2ad4f103e263f6ca702389aed46f8158a537a026ccffc94ad7e9b38391e'
    client = SRPClientSession(
            SRPContext('Pair-Setup', password='000000', prime=PRIME_3072, generator=PRIME_3072_GEN,
                       hash_func=hashlib.sha512), private=private)
    ctx = client._context
    print(ctx.pad(ctx._gen).hex())
    print(f'CKEY {client.public}')
    r = client.process(
        other_public='b06fbc51747050e9ab5af0843c1be8e96d984b4369668f5edb00fdceacc01ff622077781096e8430f585d9b3423abc884ee881d1c290274799168276f81f19a6e7f6ffd7f92fab56378357f004b556a974df3bc35924185cd12d5bfd12c213a33b697126a52af0931a23633fb983bb5bb314dc975246c97f08c1f8d191328c3c6c',
        salt='f4f1368f61e6d9ce8ebd130928d18f50',
    )
    print(f'KEY {int_to_bytes(r[0]).hex()}')
    print(f'PRF {int_to_bytes(r[1]).hex()}')
    salt = binascii.unhexlify('f4f1368f61e6d9ce8ebd130928d18f50')
    print(f'PWHS {int_to_bytes(ctx.get_common_password_hash(salt)).hex()}')
    print(f'k {int_to_bytes(ctx._mult).hex()}')
    print(f'_common_secret {int_to_bytes(client._common_secret).hex()}')
    

if __name__ == '__main__': main()
