#!/usr/bin/env bash

if [[ -n ${SSH_CONNECTION} && $- == *i* ]] ; then
    fastfetch
fi
