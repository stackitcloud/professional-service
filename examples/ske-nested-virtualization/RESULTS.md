```shell
kubectl -n kvm-validator logs -l app.kubernetes.io/name=kvm-validator --prefix

# Timestamp : 2026-08-25T10:36:38Z
#
# [PASS] /dev/kvm present crw-rw---- 1 root 109 10, 232 Aug 25 10:36 /dev/kvm
# [PASS] CPU virt flag Intel VT-x (vmx)
# [PASS] /dev/kvm access read/write confirmed
#
# Result: PASS — nested virtualisation is operational
#
# [10:36:38Z] pool=g2-intel node=shoot--d42b1685b9--nested-virt-g2-intel-z1-5d858-57c78 result=PASS
# [10:37:38Z] pool=g2-intel node=shoot--d42b1685b9--nested-virt-g2-intel-z1-5d858-57c78 result=PASS
#
# [FAIL] /dev/kvm present device not found
# [FAIL] CPU virt flag no vmx or svm in /proc/cpuinfo
# [FAIL] /dev/kvm access not accessible
#
# Result: FAIL — this flavor does not expose /dev/kvm
# Use a g2 or g3 node pool for KVM workloads.
#
# [10:36:38Z] pool=c2a-amd-no-kvm node=shoot--d42b1685b9--nested-virt-c2a-amd-no-kvm-z1-5cff9-fgvqs result=FAIL
# [10:37:38Z] pool=c2a-amd-no-kvm node=shoot--d42b1685b9--nested-virt-c2a-amd-no-kvm-z1-5cff9-fgvqs result=FAIL
# Timestamp : 2026-08-25T10:36:38Z
#
# [PASS] /dev/kvm present crw-rw---- 1 root 109 10, 232 Aug 25 10:36 /dev/kvm
# [PASS] CPU virt flag Intel VT-x (vmx)
# [PASS] /dev/kvm access read/write confirmed
#
# Result: PASS — nested virtualisation is operational
#
# [10:36:38Z] pool=g3-intel node=shoot--d42b1685b9--nested-virt-g3-intel-z1-5b768-whbzx result=PASS
# [10:37:38Z] pool=g3-intel node=shoot--d42b1685b9--nested-virt-g3-intel-z1-5b768-whbzx result=PASS
```
