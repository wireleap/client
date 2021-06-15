# Copyright (c) 2021 Wireleap

CC = gcc
CFLAGS = -D_GNU_SOURCE -O2 -fPIC
LDFLAGS = -shared -ldl

PREFIX = /usr/local
LIBDIR = $(PREFIX)/lib

LIB = wireleap_intercept.so
OBJ = $(LIB:.so=.o)

all: $(LIB)

$(LIB): $(OBJ)
	$(CC) $(OBJ) $(LDFLAGS) -o $@

clean:
	rm -f $(LIB) $(OBJ)

install: all
	cp -f $(LIB) $(DESTDIR)$(LIBDIR)
	chmod 755 $(DESTDIR)$(LIBDIR)/$(LIB)

uninstall:
	rm -f $(DESTDIR)$(LIBDIR)/$(LIB)

.c.o:
	$(CC) $(CFLAGS) -c $<
