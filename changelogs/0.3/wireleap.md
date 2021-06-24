# 0.3.1

- Manual client upgrade support:
    - added `upgrade` and `rollback` commands;
    - added `init` `--force-unpack-only` flag to only overwrite
      embedded files;
    - bundled `exec` scripts are now located in `scripts/default/`;
    - version verification performed when pulling directory information
      on startup.

- UI fixes and improvements:
    - help and usage formatting and content improvements;
    - accesskeys can now be imported directly from an `https://` URL;
    - avoid new accesskeys activation on startup;
    - bundled `exec` scripts are now located in `scripts/default/`;
    - user defined `exec` scripts in `scripts/` take precedence;
    - `wireleap tun` now verifies setuid bit and root ownership.

- Default `circuit.hops` changed to 1.
- Fixed issue where some errors during initial splice weren't reported.

# 0.3.0

- Rebranded to Wireleap (formerly Spydermix):
    - Binary name changed: `spyder` to `wireleap`.
    - Filenames used changed:
        - `spyder.pid` to `wireleap.pid`;
        - `spyder.log` to `wireleap.log`.
    - Relay URL scheme changed: `spyder://` to `wireleap://`.
    - Embedded asset filenames changed:
        - `spydertun` to `wireleap_tun`;
        - `libspydercept.so` to `wireleap_intercept.so`.
    - Script-exported environment variable name changed: `SPYDER_SOCKS`
      to `WIRELEAP_SOCKS`.
    - Target protocol environment variable name changed:
      `SM_TARGET_PROTOCOL` to `WIRELEAP_TARGET_PROTOCOL`.
- `pubkey.json` replaced with `contract.json`, now contains entire
  contract `/info` snapshot instead of just the public key.
- Address configuration changed:
    - `socks_addr` config option now `address.socks`, default value
      `127.0.0.1:13491`;
    - `tun_addr` config option now `address.tun`, default value
      "10.13.49.0:13493";
    - `h2c_addr` config option now `address.h2c`, default value
      "127.0.0.1:13492".

