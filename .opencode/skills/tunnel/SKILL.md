---
name: tunnel
description: Use when discussing or working on the CoreDevice tunnel mechanism in ix — including TUN interface, TLS-PSK, remotepair, tunneld, RSD, USB/WiFi paths, or the abstraction layer design.
---

# CoreDevice Tunnel 机制

## 核心设计思想

iOS 17+ 所有上层服务（RSD / DVT / AFC / CoreDevice …）都通过 tunneld 建立的 TUN 虚拟网卡访问设备，无论底层走 USB 还是 WiFi。

TUN 是**唯一入口**，上层服务只做 `net.Dial(设备 IPv6)`，完全不感知底层传输方式。

两条路径都需要配对：
- **WiFi**：RemotePairing（SRP + ECDH + PSK），pair record 存于 `~/.ix/`
- **USB**：lockdown 配对（读取 pair record + mTLS session），再 `StartService(CoreDeviceProxy)`

### USB 访问权限分层

USB 连接本身只提供最基础的 usbmuxd 通道，访问能力取决于配对状态：

| 状态 | 可访问 | 不可访问 |
|------|--------|----------|
| 未配对（仅 usbmux 连接） | `QueryType` 等极少数接口 | 设备数据、服务启动、文件系统 |
| 配对后（lockdown mTLS session） | 完整 lockdown 服务、`StartService`、设备信息、AFC、DVT … | — |

因此 `lockdown.New()` 是 USB 路径上一切有意义操作的前提，缺少 pair record 或配对未被信任时无法继续。

---

## 整体架构

```mermaid
graph TB
    subgraph 上层服务
        A[RSD / DVT / AFC / CoreDevice ...]
    end

    subgraph 抽象层
        B[TUN 虚拟网卡\nutun0 ~ utunN\nIPv6 /64 独立地址空间]
    end

    subgraph 传输层_WiFi
        C[RemotePairing\nSRP+ECDH+PSK]
        C2[TCP + TLS-PSK\nAES-256-GCM]
    end

    subgraph 传输层_USB
        D1[lockdown\nmTLS 配对]
        D2[CoreDeviceProxy\nCDTunnel 握手]
    end

    subgraph 物理层
        E[WiFi]
        F[USB / usbmux]
    end

    A -->|net.Dial 设备 IPv6| B
    B -->|IPv6 原始包转发| C2
    B -->|IPv6 原始包转发| D2
    C --> C2
    C2 --> E
    D1 --> D2
    D2 --> F
```

---

## WiFi 路径：RemotePairing + TCP + PSK

```mermaid
sequenceDiagram
    participant Mac
    participant Device as iPhone

    Note over Mac,Device: 1. 配对协议 (首次)
    Mac->>Device: XPC over H2C (RemoteXPC)
    Mac->>Device: SRP-3072 PIN 验证 (000000)
    Mac->>Device: ECDH X25519 密钥协商
    Mac->>Device: Ed25519 身份签名
    Device-->>Mac: encryptKey (共享密钥)
    Mac->>Mac: 保存 PairRecord (~/.ix/RP_<UDID>.plist)

    Note over Mac,Device: 2. 建立 TCP Tunnel
    Mac->>Device: createListener(tcp, key=encryptKey)
    Device-->>Mac: port (动态端口)
    Mac->>Device: TCP Dial port
    Mac->>Device: TLS-PSK 握手\nTLS_PSK_WITH_AES_256_GCM_SHA384
    Device-->>Mac: PSK handshake OK

    Note over Mac,Device: 3. CDTunnel 握手
    Mac->>Device: clientHandshakeRequest (mtu=16000)
    Device-->>Mac: clientIP, serverIP, serverRSDPort, mtu

    Note over Mac: 4. 建立 TUN
    Mac->>Mac: gost.TunListener\n配置 clientIP/MTU → utunN
    Mac->>Device: IPv6 包双向转发

    Note over Mac: 5. RSD 连接
    Mac->>Device: xpc.NewRemoteXpc(serverIP:serverRSDPort)\n通过 utunN 路由
```

---

## USB 路径：lockdown 配对 + CoreDeviceProxy

```mermaid
sequenceDiagram
    participant Mac
    participant usbmuxd
    participant Device as iPhone

    Mac->>usbmuxd: 连接 /var/run/usbmuxd
    usbmuxd-->>Mac: Attached (UDID, DeviceID)

    Mac->>usbmuxd: lockdown Connect (port 32498)
    usbmuxd->>Device: 端口转发
    Mac->>Device: ReadPairRecord + StartSession (mTLS)
    Mac->>Device: StartService(CoreDeviceProxy)
    Device-->>Mac: service port

    Note over Mac,Device: 直接在 lockdown socket 上跑 CDTunnel
    Mac->>Device: clientHandshakeRequest (mtu=16000)
    Device-->>Mac: clientIP, serverIP, serverRSDPort, mtu

    Note over Mac: 建立 TUN
    Mac->>Mac: gost.TunListener → utunN
    Mac->>Device: IPv6 包双向转发 (经 usbmux)

    Mac->>Device: xpc.NewRemoteXpc(serverIP:serverRSDPort)\n通过 utunN 路由
```

---

## tunneld 服务发现与生命周期

```mermaid
stateDiagram-v2
    [*] --> 监听

    state 监听 {
        Bonjour_WiFi: Bonjour mDNS\n(_remoted / _remotepairing)
        USB监听: usbmux.Listen\nAttached / Detached
    }

    监听 --> WiFi路径: mDNS 发现设备 IP
    监听 --> USB路径: Attached 事件

    state WiFi路径 {
        rsd_new: rsd.New(ip:58783)\nRemoteXPC 握手
        remotepair: remotepair.NewFromRSD\nSRP+ECDH+Ed25519
        tcp_tunnel: StartTcpTunnel\nNewPSKConn
    }
    rsd_new --> remotepair
    remotepair --> tcp_tunnel

    state USB路径 {
        lockdown: lockdown.New\nusbmux 端口转发
        coredeviceproxy: StartService\nCoreDeviceProxy
        tunnel_new: tunnel.New + Start\nCDTunnel 握手
    }
    lockdown --> coredeviceproxy
    coredeviceproxy --> tunnel_new

    tcp_tunnel --> TUN激活
    tunnel_new --> TUN激活

    TUN激活 --> RSD连接: rsd.NewFromTunnel\n设备 IPv6:58783
    RSD连接 --> HTTP注册: svcs[udid] / usbTuns[udid]
    HTTP注册 --> 可被查询: GET /rsd/:udid

    可被查询 --> 设备断开: Detached / 连接中断
    设备断开 --> TUN销毁: tun.Stop()
    TUN销毁 --> [*]
```

---

## TLS-PSK 握手详情 (TCP 路径专用)

```mermaid
sequenceDiagram
    participant Client as Mac (tlsPSKConn)
    participant Server as iPhone

    Client->>Server: ClientHello\n cipher=TLS_PSK_WITH_AES_256_GCM_SHA384\n renegotiation_info
    Server-->>Client: ServerHello (选定 0x00A9)\n renegotiation_info
    Server-->>Client: ServerHelloDone
    Client->>Server: ClientKeyExchange (空 PSK identity)

    Note over Client,Server: 双方用 encryptKey 独立推导
    Note over Client,Server: premaster = 0x00..00 ‖ PSK
    Note over Client,Server: master = PRF-SHA384(premaster, randoms)
    Note over Client,Server: key_block → clientKey/serverKey/IVs

    Client->>Server: ChangeCipherSpec
    Client->>Server: Finished (PRF-SHA384 verify_data, 加密)
    Server-->>Client: ChangeCipherSpec
    Server-->>Client: Finished (加密)

    Note over Client,Server: 握手完成\n后续 ApplicationData 用 AES-256-GCM 加密
```

---

## 传输模式对比

```mermaid
graph LR
    subgraph iOS_18_2之前
        Q[QUIC / UDP\nRSA 自签名证书]
        T1[TCP\nTLS-PSK]
    end

    subgraph iOS_18_2之后
        T2[TCP\nTLS-PSK ✅ 唯一路径]
        X[QUIC ❌ 已移除]
    end

    Q -.->|已废弃| X
    T1 -->|保留| T2
```

iOS 18.2+ QUIC 路径被 Apple 移除，TCP + PSK 是**唯一可用**的 WiFi tunnel 传输方式。

实测（iOS 26.6.1 / iPhone16,2）：QUIC 握手返回 `CRYPTO_ERROR 0x128`（TLS handshake_failure），设备直接拒绝，自动降级到 TCP+PSK 正常工作。

---

## 关键文件索引

| 文件 | 职责 |
|---|---|
| `api/remotepair/tlspsk.go` | 纯 Go TLS-PSK 实现（TLS 1.2，0x00A9） |
| `api/remotepair/pskconn.go` | `NewPSKConn` 入口，调 `newPSKConn` |
| `api/remotepair/remotepair.go` | SRP/ECDH/Ed25519 配对 + `StartTcpTunnel` |
| `api/remotepair/connection.go` | `wirePairConnection` / `wifiPairConnection` |
| `api/tunnel/tunnel.go` | CDTunnel 握手 + TUN 创建 + IPv6 转发 |
| `api/tunnel/rsd/rsd.go` | RSD 连接（通过 TUN 后的设备 IPv6） |
| `api/tunnel/xpc/remote.go` | RemoteXPC over H2C |
| `api/tunnel/h2c/h2c.go` | 裸 HTTP/2 帧层（非标准 HTTP） |
| `api/tunneld/tunneld.go` | daemon：Bonjour + USB 发现，管理 tunnel 生命周期 |
| `api/lockdown/lockdown.go` | USB lockdown 配对 + StartService |
