#!/usr/bin/env bash

if [[ -n ${SSH_CONNECTION} && $- == *i* ]] ; then
    neofetch
fi
