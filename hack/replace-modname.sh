#!/usr/bin/env bash

OLD_MOD="code.alipay.com/dbplatform/node-disk-controller"
NEW_MOD="github.com/eosphoros-ai/liteio"

find . -name "*.go" -type f -exec sed -i -e "s|$OLD_MOD|$NEW_MOD|g" {} \;
find . -name "Makefile" -type f -exec sed -i -e "s|$OLD_MOD|$NEW_MOD|g" {} \;