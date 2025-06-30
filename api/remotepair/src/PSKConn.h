//
//  PskConn.hpp
//  PSKConn
//
//  Created by larryhou on 2025/6/25.
//

#ifndef APPLETUNNELD_PSKCONN
#define APPLETUNNELD_PSKCONN

#ifdef __cplusplus
extern "C" {
#endif

const char* PSK_newConn(int fd, const char* key, void** h);
int PSK_write(void* h, void* buf, int num);
int PSK_read (void* h, void* buf, int num);
void PSK_close(void* h);

#ifdef __cplusplus
}
#endif

#endif /* APPLETUNNELD_PSKCONN */
