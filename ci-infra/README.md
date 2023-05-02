# Introduction

This directory contains the sources to run ci infra for respective cloud providers.

## How to run and setup CI

Refer to specific cloud providers docs under respective folders

## How to update CI workflow

To provide a way to access repository secrets in PRs from forked repo there are some restrictions to update
CI workflow.

### Steps to update CI work 
1. Merge all the changes other than CI workflow changes in first PR.
2. Make changes specific to only CI workflow in second PR. 
3. Test this second PR CI changes on personal public forked repo and reference test results from CI run in personal repo in this PR.
4. Get this PR merged on upstream main branch.

> **NOTE**: As this needs to access repository secrets so we cant run changes in CI without merging it refer to [this](https://iterative.ai/blog/testing-external-contributions-using-github-actions-secrets) for more info.

