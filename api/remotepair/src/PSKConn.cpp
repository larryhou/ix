//
//  PSKConn.cpp
//  PSKConn
//
//  Created by larryhou on 2025/6/25.
//

#include "PSKConn.h"

#include <openssl/ssl.h>
#include <openssl/err.h>
#include <unistd.h>
#include <arpa/inet.h>
#include <cstring>
#include <mutex>
#include <map>


namespace {
struct Context {
    int fd;
    SSL_CTX* ctx;
    SSL* ssl;
    const char* key;
    bool connected;
};

std::map<SSL*, Context*> registry;
std::mutex registry_mutex;
}

unsigned int psk_client_callback(SSL *ssl, const char *hint,
                           char *identity, unsigned int max_identity_len,
                           unsigned char *psk, unsigned int max_psk_len) {
    const char* hexkey = nullptr;
    {
        std::lock_guard<std::mutex> lock(registry_mutex);
        auto it = registry.find(ssl);
        if (it != registry.end()) {
            hexkey = it->second->key;
        } else {
            return 0;
        }
    }
    long psk_len = 0;
    auto key = OPENSSL_hexstr2buf(hexkey, &psk_len);
    if (!psk_len || !key) {
        return 0;
    }
    memcpy(psk, key, psk_len);
    OPENSSL_free(key);
    return static_cast<unsigned int>(psk_len);
}

// local_ip 改为 iface，支持通过网卡名绑定
const char* PSK_newConn(int fd, const char* key, void** h) {
    static bool initialized = false;
    if (!initialized) {
        initialized = true;
        SSL_library_init();
        SSL_load_error_strings();
    }
    auto p = new Context();
    memset(p, 0, sizeof(Context));
    auto m = TLS_client_method();
    p->ctx = SSL_CTX_new(m);
    p->fd = fd;
    if (!p->ctx) {
        return "SSL_CTX_new error";
    }
    SSL_CTX_set_cipher_list(p->ctx, "PSK");
    SSL_CTX_set_psk_client_callback(p->ctx, psk_client_callback);
    SSL_CTX_set_max_proto_version(p->ctx, TLS1_2_VERSION);
    SSL_CTX_set_min_proto_version(p->ctx, TLS1_2_VERSION);
    
    p->key = strdup(key); // 拷贝 key，防止悬挂
    p->ssl = SSL_new(p->ctx);
    SSL_set_fd(p->ssl, p->fd);
    {
        std::lock_guard<std::mutex> lock(registry_mutex);
        registry[p->ssl] = p;
    }
    
    if (SSL_connect(p->ssl) != 1) {
        PSK_close(p);
        return "SSL_connect error";
    }
    p->connected = true;
    *h = p;
    return nullptr;
}

int PSK_write(void* h, void* buf, int num) {
    auto p = (Context*)h;
    auto n = SSL_write(p->ssl, buf, num);
    if (n < 0) {
        n = -SSL_get_error(p->ssl, n);
    }
    
    return n;
}

int PSK_read(void* h, void* buf, int num) {
    auto p = (Context*)h;
    auto n = SSL_read(p->ssl, buf, num);
    if (n < 0) {
        n = -SSL_get_error(p->ssl, n);
    }
    
    return n;
}

void PSK_close(void* h) {
    if (h != nullptr) {
        auto p = (Context*)h;
        if (p->connected) {
            SSL_shutdown(p->ssl);
            p->connected = false;
        }
        {
            std::lock_guard<std::mutex> lock(registry_mutex);
            registry.erase(p->ssl); // 释放注册表
        }
        SSL_free(p->ssl);
        SSL_CTX_free(p->ctx);
        free((void*)p->key); // 释放 key
        delete p;
    }
}
