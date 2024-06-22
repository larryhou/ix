#!/usr/bin/env python
from pymobiledevice3.remote.remotexpc import RemoteXPCConnection
from pymobiledevice3.remote.remote_service_discovery import RemoteServiceDiscoveryService
import asyncio
import time

def main():
    rc = RemoteServiceDiscoveryService(('fe80::fc5d:4ff:fecd:10a3%en6', 58783))
    loop = asyncio.get_event_loop()
    loop.run_until_complete(asyncio.wait([
        loop.create_task(rc.connect())
    ]))
    print(rc.peer_info)

if __name__ == '__main__': main()
