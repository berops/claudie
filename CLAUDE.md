# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Claudie builds and manages multi-cloud, hybrid and on-premises Kubernetes clusters, with its own load balancers and DNS, from a single `InputManifest` custom resource (`claudie.io/v1beta1`). It runs in an existing management cluster that needs cert-manager for the operator's validating webhook. Deleting an InputManifest destroys all infrastructure it created.

## E2E Tests
- E2E tests live in [test/e2e](test/e2e). They replace the legacy framework in [services/testing-framework](services/testing-framework) (`make test`, test sets in [manifests/testing-framework/test-sets](manifests/testing-framework/test-sets)). Don't extend the legacy framework or copy its patterns.
- Each `*_test.go` file covers one scenario.
- They run only in the CI pipeline.
- They create real, paid infrastructure. Every test MUST delete its InputManifest so the infrastructure is destroyed.
- If all tests pass, [.github/workflows/ci.yml](.github/workflows/ci.yml) deletes the Claudie namespace in the e2e cluster. If any test fails, the namespace is kept for manual investigation.
- Use the official client-go library to talk to Kubernetes, not the legacy [internal/kubectl](internal/kubectl) wrapper. Each call made with the k8s client should have three retries. Calls sometimes fail, so we want to retry them instead of failing the test right away.
- Setup helpers fail the test themselves.
- Assertions belong in the test function, not in helpers. Keep them one-line: a small function returns the value and the test checks it, e.g. `if got := ReadyNodeCount(t, client); got != want { t.Fatalf(...) }`. Don't inline loops or counting into the test.
- Failure messages print got and want, with context.
- Use descriptive, idiomatic Go names instead of comments. Comment only in exceptional cases, and try to keep comments concise: prefer one-line comments, two lines at most.
