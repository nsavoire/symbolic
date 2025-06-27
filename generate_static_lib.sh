#!/usr/bin/env bash

set -e

# Build the symbolic library
cargo build --release --package symbolic-cabi

if [ $(uname) == "Darwin" ]; then
  mkdir -p target/$(uname -m)-apple-darwin/release
  /usr/bin/nm --defined-only --extern-only --no-llvm-bc  target/release/libsymbolic_cabi.a | grep -o '_symbolic_.*' > symbolic.syms
  cc -v -nodefaultlibs -r -o target/$(uname -m)-apple-darwin/release/libsymbolic_cabi.a -Wl,-exported_symbols_list,symbolic.syms target/release/libsymbolic_cabi.a
else
  mkdir -p target/$(uname -m)-unknown-linux/release
  ld -r --whole-archive target/release/libsymbolic_cabi.a -o target/release/libsymbolic_cabi.o
  objcopy -g --wildcard --keep-global-symbol="symbolic_*" target/release/libsymbolic_cabi.o target/$(uname -m)-unknown-linux/release/libsymbolic_cabi.a
fi
