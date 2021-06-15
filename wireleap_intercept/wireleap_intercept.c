/* Copyright (c) 2021 Wireleap */

#include <arpa/inet.h>
#include <dlfcn.h>
#include <errno.h>
#include <fcntl.h>
#include <netdb.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define WL_ADDR "localhost"
#define WL_PORT "13491"

static struct hostent *(*o_gethostbyname)(const char *name);
static int (*o_getaddrinfo)(const char *node, const char *service,
		const struct addrinfo *hints, struct addrinfo **res);
static int (*o_connect)(int sockfd, const struct sockaddr *addr,
		socklen_t addrlen);

static struct addrinfo *proxy_info;
static char saved_node[256];

/* Initializer, intercept functions and initialize the SOCKS proxy */
static void __attribute__((constructor))
init(void)
{
	char *env_socks_proxy, *proxy_addr, *proxy_port;
	struct addrinfo hints;

	*(void **) (&o_connect) = dlsym(RTLD_NEXT, "connect");
	*(void **) (&o_getaddrinfo) = dlsym(RTLD_NEXT, "getaddrinfo");
	*(void **) (&o_gethostbyname) = dlsym(RTLD_NEXT, "gethostbyname");

	memset(&hints, 0, sizeof(struct addrinfo));
	hints.ai_family = AF_INET;
	hints.ai_socktype = SOCK_STREAM;
	hints.ai_flags = 0;
	hints.ai_protocol = 0;

	env_socks_proxy = getenv("SOCKS5_PROXY");
	if (env_socks_proxy) {
		proxy_addr = strtok(env_socks_proxy, ":");
		proxy_port = strtok(NULL, ":");
	} else {
		proxy_addr = WL_ADDR;
		proxy_port = WL_PORT;
	}
	o_getaddrinfo(proxy_addr, proxy_port, &hints, &proxy_info);
}

/* Intercepted getaddrinfo(3) */
int
getaddrinfo(const char *node, const char *service,
		const struct addrinfo *hints, struct addrinfo **res)
{
	strncpy(saved_node, node, sizeof(saved_node)-1);
	return (*o_getaddrinfo)("0.0.0.1", service, hints, res);
}

/* Intercepted gethostbyname(3) */
struct hostent
*gethostbyname(const char *name)
{
	strncpy(saved_node, name, sizeof(saved_node)-1);
	return (*o_gethostbyname)("0.0.0.1");
}


/* SOCKS proxy */
int
connect_proxy(int sockfd, const struct sockaddr_in *addr, socklen_t addrlen)
{
	int ret, fd_flags;
	struct sockaddr_in *proxy_addr;
	char buf[512];

	fd_flags = fcntl(sockfd, F_GETFL, 0);
	fcntl(sockfd, F_SETFL, fd_flags & ~O_NONBLOCK);

	ret = (*o_connect)(sockfd, proxy_info->ai_addr, sizeof(struct sockaddr));
	if (ret != 0) {
		proxy_addr = (struct sockaddr_in *)(proxy_info->ai_addr);
		inet_ntop(proxy_addr->sin_family, &proxy_addr->sin_addr, buf,
				sizeof(buf));
		return ret;
	}

	send(sockfd, "\x05\x01\x00", 3, 0);
	recv(sockfd, buf, 2, 0);
	if (memcmp(buf, "\x05\x00", 2) != 0) {
		errno = ECONNREFUSED;
		return -1;
	}

	if (memcmp(&addr->sin_addr, "\x00\x00\x00\x01", 4) == 0) {
		int nodelen = strlen(saved_node);
		if (nodelen > sizeof(saved_node) || 7+nodelen > sizeof(buf)) {
			errno = ECONNREFUSED;
			return -1;
		}
		memcpy(buf, "\x05\x01\x00\x03", 4);
		buf[4] = (unsigned char)nodelen;
		memcpy(buf+5, saved_node, nodelen);
		memcpy(buf+5+nodelen, &addr->sin_port, 2);
		send(sockfd, buf, 7+nodelen, 0);
	} else if (addr->sin_family == AF_INET) {
		memcpy(buf, "\x05\x01\x00\x01", 4);
		memcpy(buf+4, &addr->sin_addr, 4);
		memcpy(buf+8, &addr->sin_port, 2);
		send(sockfd, buf, 10, 0);
	} else if (addr->sin_family == AF_INET6) {
		memcpy(buf, "\x05\x01\x00\x04", 4);
		memcpy(buf+4, &addr->sin_addr, 16);
		memcpy(buf+20, &addr->sin_port, 2);
		send(sockfd, buf, 22, 0);
	} else {
		exit(1);
	}

	recv(sockfd, buf, 10, 0);
	if (memcmp(buf, "\x05\x00", 2) != 0) {
		errno = ECONNREFUSED;
		return -1;
	}

	fcntl(sockfd, F_SETFL, fd_flags);
	return ret;
}

/* Intercepted connect(3) function */
int
connect(int sockfd, const struct sockaddr *addr, socklen_t addrlen)
{
	int so_type;
	socklen_t optlen;
	struct sockaddr_in *addr_in;

	if (addr->sa_family == AF_INET || addr->sa_family == AF_INET6) {
		char str_address[64];
		optlen = sizeof(so_type);
		getsockopt(sockfd, SOL_SOCKET, SO_TYPE, &so_type, &optlen);

		addr_in = (struct sockaddr_in *)addr;
		inet_ntop(addr_in->sin_family, &addr_in->sin_addr, str_address,
				sizeof(str_address));
		if (so_type & SOCK_STREAM) {
			return connect_proxy(sockfd, addr_in, addrlen);
		}
	}
	return (*o_connect)(sockfd, addr, addrlen);
}
