<!-- tags: iaas, windows, byol, migration, compute -->

# **Guide: BYOL Migration to STACKIT**

> ⚠️ Example images are still in German. Translating them into English is an open TODO.

This document provides a migration path for your custom-built Windows Server VM (Bring Your Own License) from a local virtualization environment (e.g., Hyper-V / VirtualBox) to the STACKIT cloud platform.

The detailed process ensures technical compatibility through the integration of VirtIO drivers and the conversion of disk images. Following these steps allows you to use your own Windows licenses within the STACKIT cloud.

---

### **Prerequisites**

To successfully complete this workflow, you need access to the following tools and resources:

- **STACKIT Windows VM (Recommended Sizing)**
  - Flavor G2i.8
  - Disk OS Perf6 - 64GB
  - Data/Image Disk Perf10: 100GB
- **Hyper-V:** Install as a virtualization platform via the Windows Role/Feature (e.g., via Server Manager).
- **Qemu-img:** [https://www.qemu.org/download/#windows](https://www.qemu.org/download/#windows)
- **STACKIT CLI:** [https://github.com/stackitcloud/stackit-cli/releases](https://github.com/stackitcloud/stackit-cli/releases)
- **Virtio Drivers:** [https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/?C=M;O=D](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/?C=M;O=D)
- **Cloud-base Init:** [https://github.com/cloudbase/cloudbase-init/releases](https://github.com/cloudbase/cloudbase-init/releases)

---

### **Step-by-Step Migration**

1. **Set up a new VM:** Action (**Aktion**) → New (**Neu**) → Virtual Machine (**Virtueller Computer**)
   <p align="center"><img src="images/image1.jpg" width="650"></p>

2. Click **Next** (**Weiter**)
   <p align="center"><img src="images/image2.jpg" width="650"></p>

3. **Specify Name and Location:** Enter the name of the new VM and, if necessary, a different storage location → **Next** (**Weiter**)
   <p align="center"><img src="images/image3.jpg" width="650"></p>

4. Select **Generation 2**

   > **Note:** With Generation 2, you must manually press "Any Key" during startup to boot from the ISO image. If you miss this moment, the installation routine will not start!

   <p align="center"><img src="images/image4.jpg" width="650"></p>

5. **Assign Memory:** Startup memory (**Arbeitsspeicher beim Start**) → Enter value as needed (e.g., 4096 MB).
   **Uncheck** the box: Use Dynamic Memory for this virtual machine (**Dynamischen Arbeitsspeicher für diesen virtuellen Computer verwenden**)
   <p align="center"><img src="images/image5.jpg" width="650"></p>

6. **Configure Networking:** Connection (**Verbindung**) → Not Connected (**Nicht verbunden**)
   <p align="center"><img src="images/image6.jpg" width="650"></p>

7. **Connect Virtual Hard Disk:** Define Name, Location (**Pfad**), and Size (**Größe**)
   <p align="center"><img src="images/image7.jpg" width="650"></p>
   The configured size corresponds to the minimum volume size of the future server in STACKIT.

8. **Installation Options:** Install an operating system from a bootable CD/DVD-ROM (**Betriebssystem von einer startbaren CD/DVD-ROM installieren**) → Select Image file (**Abbilddatei (ISO)**) and use Browse (**Durchsuchen**) to select the required ISO image.
   <p align="center"><img src="images/image8.jpg" width="650"></p>

9. **Finish the New Virtual Machine Wizard:** Click **Finish** (**Fertig stellen**)
   <p align="center"><img src="images/image9.jpg" width="650"></p>

10. **Hyper-V Manager** view after creating the new VM
    <p align="center"><img src="images/image10.jpg" width="650"></p>

11. **Attach the Virtio drivers via ISO:**
    <p align="center"><img src="images/image11.jpg" width="650"></p>

12. Click **Connect** (**Verbinden**) to the new VM
    <p align="center"><img src="images/image12.jpg" width="650"></p>
    <p align="center"><img src="images/image13.jpg" width="650"></p>

13. Start the new VM for the first time and perform the OS installation
    <p align="center"><img src="images/image14.jpg" width="650"></p>

14. **Perform Windows Server Setup** (Screenshots based on Windows Server 2022):
    <p align="center"><img src="images/image15.jpg" width="650"></p>
    <p align="center"><img src="images/image16.jpg" width="650"></p>

15. <p align="center"><img src="images/image17.jpg" width="650"></p>

16. <p align="center"><img src="images/image18.jpg" width="650"></p>

17. <p align="center"><img src="images/image19.jpg" width="650"></p>

18. Use the **Load Driver** (**Treiber laden**) selection
    <p align="center"><img src="images/image20.jpg" width="650"></p>

19. Installation of **three** Virtio drivers is now required so the image can be used on the STACKIT Hypervisor:
    <p align="center"><img src="images/image21.jpg" width="650"></p>

    **NetKVM Driver**
    <p align="center"><img src="images/image22.jpg" width="650"></p>
    <p align="center"><img src="images/image23.jpg" width="650"></p>

    **Viostor**
    <p align="center"><img src="images/image24.jpg" width="650"></p>
    <p align="center"><img src="images/image41.jpg" width="650"></p>

    **Vioscsi**
    <p align="center"><img src="images/image24.jpg" width="650"></p>
    <p align="center"><img src="images/image25.jpg" width="650"></p>

20. <p align="center"><img src="images/image26.jpg" width="650"></p>

21. <p align="center"><img src="images/image27.jpg" width="650"></p>

22. <p align="center"><img src="images/image28.jpg" width="650"></p>

23. **Display Configuration**
    <p align="center"><img src="images/image29.jpg" width="650"></p>

24. The two Virtio packages (**virtio-win-gt-x64.msi** and **virtio-win-guest-tools.exe**) from the Virtio ISO file should now be installed. It is also recommended to copy the content of the Virtio ISO file to the new system (e.g., `C:\temp\virtio\`). This has the advantage of being able to reinstall drivers relatively easily later.

25. **Delete the Windows Recovery Partition**
    This step is mandatory so that the volume of the future server on STACKIT can be flexibly expanded.

| Step  | Command                     | Details / Notes                                                   |
| :---- | :-------------------------- | :---------------------------------------------------------------- |
| **1** | `diskpart`                  | Starts the partitioning program.                                  |
| **2** | `select disk 0`             | Selects the hard disk. **Be sure to check** if Disk 0 is correct! |
| **3** | `list partition`            | Displays all existing partitions.                                 |
| **4** | `select partition <nr>`     | Select the number of the Recovery partition.                      |
| **5** | `delete partition override` | Forces the deletion of the partition.                             |
| **6** | `list partition`            | Check if the partition was successfully removed.                  |

26. The Windows system can now be customized with individual software and prepared for the future image.

27. Finally, run the [**Cloudbase-init Tool**](https://cloudbase.it/cloudbase-init/) on the Windows VM to bring Windows into the final starting position for the move to the STACKIT Cloud!

28. Start **Cloudbase-Init Setup**
    <p align="center"><img src="images/image30.jpg" width="650"></p>

29. Agree to the **License Agreement** (**Lizenzvereinbarung**)
    <p align="center"><img src="images/image31.jpg" width="650"></p>

30. Confirm **Setup Type**
    <p align="center"><img src="images/image32.jpg" width="650"></p>

31. Define **Configuration Options**
    <p align="center"><img src="images/image33.jpg" width="650"></p>

32. Start **Installation**
    <p align="center"><img src="images/image34.jpg" width="650"></p>

33. Finish installation and execute **Sysprep** (**Sysprep ausführen**)
    <p align="center"><img src="images/image35.jpg" width="650"></p>

34. **Sysprep generalization** is running
    <p align="center"><img src="images/image36.jpg" width="650"></p>

---

### **35. Image-Upload & VM Creation in STACKIT**

After the local preparation is complete, the image is converted and transferred via STACKIT CLI.

#### **36. Image Conversion (qCow2)**

Convert the local VHDX into qcow2 format:

````bash
qemu-img convert -f vhdx -O qcow2 <Path_to_vhdx> <Path_to_qcow2>

#### 37. STACKIT CLI Login
Authenticate at the CLI:

```bash
stackit auth login
````

#### 38. Image Upload

Upload the image to your STACKIT project:

```bash
stackit image create --name <win2025virtio> --disk-format=qcow2 --local-file-path="<path2qcow2>" -p <projectID>
```

#### 39. Status Check

Check the upload progress and details:

```bash
stackit image list -p <projectID>
stackit image describe <imageID> -p <projectID>
```

> **Important:** Take the generated `imageID` from the output. You must specify this ID as `<image_id>` in the next step to create the volume and the VM based on this image.

#### 40. Provisioning (Volume & Server)

First create the volume and then start the VM:

**Step 1: Create Volume**

```bash
stackit volume create --availability-zone <AZ> \
--name <volumename> --source-id <image_id> \
--source-type image --size <GB> -p <projectID>
```

**Step 2: Instantiate Server**

```bash
stackit server create -n <servername> \
--availability-zone <AZ> --machine-type <machineType> \
--network-id <networkID> --boot-volume-source-id <volumeID> \
--boot-volume-source-type volume -p <projectID>
```

#### 41. Image Sharing (Cross-Project)

Share the image for other Project IDs within the organization:

```bash
stackit curl -X PATCH -H "Content-Type: application/json" \
--data '{"projects": ["<ID1>", "<ID2>"]}' \
https://iaas.api.eu01.stackit.cloud/v1/projects/<PROJECT_ID>/images/<IMAGE_ID>/share
```

#### 42. Completion

Check if all drivers are correctly loaded in the operating system.
After starting the VM in STACKIT, check the **Device Manager** (**Gerätemanager**) to verify that all drivers have been loaded properly.

<p align="center"><img src="images/image37.jpg" width="650"></p>

References:
[https://docs.stackit.cloud/stackit/en/create-a-windows-server-via-stackit-iaas-api-cli-98304598.html](https://docs.stackit.cloud/stackit/en/create-a-windows-server-via-stackit-iaas-api-cli-98304598.html)
