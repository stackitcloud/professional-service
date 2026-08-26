<!-- tags: ske, encryption, kms, block-storage, kubernetes, csi -->

# Encrypted Volumes for SKE

> ⚠️This example assumes that your project or organization has been enabled for a preview version of the STACKIT CSI Driver. If you wish to use encrypted volumes, please contact your account manager.

## Overview

This guide demonstrates how to roll out an encrypted storage class for SKE using the STACKIT Key Management Service (KMS). To achieve this, we use a **Service Account Impersonation** (Act-As) pattern. This allows the internal SKE service account to perform encryption and decryption tasks on behalf of a user-managed service account that has been granted access to your KMS keys.
