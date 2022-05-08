#! /bin/bash

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

if [[ -z "${VERSION}" ]]; then
        error_exit "Missing input for flag version"
fi

sed "s/__VERSION__/$VERSION/g" Tools/release/header.md.template >header
sed "s/__VERSION__/$VERSION/g" Tools/release/downloading-links.md.template >downloading-links
# Formats changelog sections: **<label>:** becomes ### <label>
# Skips the first 4 lines (title and version number)
sed -e 1,4d -e "s/^\*\*\(.*\)\:\*\*/### \1/g" "docs/releases/${VERSION}.md" >changelog

cat header changelog downloading-links >release-note