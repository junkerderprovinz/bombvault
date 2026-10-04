# BombVault

**ข้อมูล Unraid ของคุณ ถูกผนึกไว้ในห้องนิรภัย ปล่อยการสำรองข้อมูลลงไป จุดชนวนการกู้คืน**

BombVault คือเว็บแอปแบบ self-hosted ที่ออกแบบมาสำหรับ Unraid โดยเฉพาะ สำหรับ **การสำรองข้อมูลและการกู้คืนจากภัยพิบัติแบบเต็มรูปแบบ** ของ Docker containers และ KVM/libvirt VMs ของคุณ มันทำงานเป็น Docker container แบบ multi-arch เพียงตัวเดียว มอบเว็บ UI ที่ทันสมัยซึ่งปรับตามธีมสว่าง/มืดของระบบคุณโดยอัตโนมัติ และจัดการวงจรทั้งหมด: สำรองข้อมูล, ตั้งตารางเวลา, ตรวจสอบ และกู้คืน

การกู้คืนทำงานโดยอัตโนมัติ Containers จะปรากฏขึ้นอีกครั้งในแท็บ Docker ของ Unraid เหมือนเดิมทุกประการ และ VMs จะถูกกำหนดใหม่ใน VM Manager พร้อมกับดิสก์และ UEFI NVRAM ที่เชื่อมต่อกลับเข้าไป ไม่ต้องติดตั้งใหม่ด้วยมือ ไม่ต้องตั้งค่าใหม่ ไม่มีเรื่องวุ่นวาย

ขับเคลื่อนด้วย [restic](https://restic.net) ดังนั้นทุกการสำรองข้อมูลจึงมีการขจัดข้อมูลซ้ำ (deduplicated), เป็นแบบเพิ่มส่วน (incremental) และเข้ารหัสเสมอ

!!! note "เก็บ APP_KEY ของคุณให้ปลอดภัย"
    BombVault นำรหัสผ่านของรีพอสิทอรี restic มาจากค่าลับขนาด 32 ไบต์ชื่อ `APP_KEY` การทำหายจะทำให้การสำรองข้อมูลที่เข้ารหัสไว้ไม่สามารถกู้คืนได้ สร้างขึ้นด้วยคำสั่ง `openssl rand -hex 32` แล้วเก็บไว้ในที่ปลอดภัย ดู [Configuration](configuration.md)

## BombVault ปกป้องอะไรบ้าง

| โดเมน | สิ่งที่ถูกบันทึก |
|---|---|
| **Docker containers** | ไดเรกทอรี appdata พร้อมคำจำกัดความของ container (อิมเมจ, env vars, พอร์ต, ป้ายกำกับ, โวลุ่ม) |
| **KVM / libvirt VMs** | อิมเมจดิสก์ของ VM, คำจำกัดความ XML และ UEFI NVRAM สำรองข้อมูลผ่าน SSH (ไม่ต้องเมานต์ libvirt) |
| **Unraid flash** | แฟลช USB ทั้งหมด (`/boot`): OS, ลิขสิทธิ์, การตั้งค่าอาร์เรย์, แชร์, การตั้งค่าเครือข่ายและปลั๊กอิน |
| **การตั้งค่าแอป** | `/config` ของ BombVault เอง: ฐานข้อมูลการตั้งค่า, ข้อมูลรับรองนอกสถานที่ และคู่คีย์ SSH ของ libvirt |
| **ไฟล์และโฟลเดอร์** | **ชุดไฟล์ (file sets)** ที่ตั้งชื่อไว้, โฟลเดอร์ใดก็ได้บนเซิร์ฟเวอร์ แต่ละชุดมีรูปแบบการยกเว้นเฉพาะชุดได้ตามต้องการ |
| **ชุดข้อมูล ZFS** | ชุดข้อมูลหนึ่งพร้อมทุกชุดที่อยู่ข้างใต้ อ่านจากสแนปช็อต ZFS ครั้งเดียวและเก็บเหมือนโฟลเดอร์ ดู [ชุดข้อมูล ZFS](zfs-datasets.md) |

## การกู้คืนคือพระเอก

หลังจากคัดลอกข้อมูลกลับจากสแนปช็อต restic แล้ว BombVault จะเล่นซ้ำคำจำกัดความของ container ที่บันทึกไว้ผ่าน Docker API ดังนั้น container จึงปรากฏขึ้นอีกครั้งในแท็บ Docker ของ Unraid ราวกับว่ามันอยู่ที่นั่นตลอดมา (อิมเมจเดิม, การตั้งค่าเดิม, การแมปพอร์ตเดิม) VMs จะได้รับการกำหนด XML ใหม่ผ่าน SSH และดิสก์กับ UEFI NVRAM ถูกเชื่อมต่อกลับเข้าไป แม้ว่า VM จะถูกลบไปแล้วก็ตาม

เมื่อการสำรองข้อมูลหยุด containers ที่พึ่งพากัน พวกมันจะกลับมาในลำดับที่ถูกต้อง: BombVault จะรีสตาร์ทตามลำดับ `depends_on` ของ Compose และรอให้แต่ละตัวรายงานว่าสมบูรณ์ (healthy) ก่อนที่จะเริ่มตัวที่พึ่งพามัน ดังนั้นจึงไม่มีอะไรวิ่งแซงหน้าฐานข้อมูลหรือเกตเวย์ที่ยังไม่พร้อม ดู [Features](features.md)

## มันทำงานอย่างไร

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault ใช้ Docker socket เพื่อหยุด containers ก่อนการสำรองข้อมูลและสร้างใหม่หลังการกู้คืน สำหรับ VMs มันรัน `virsh` บนโฮสต์ผ่าน SSH (`qemu+ssh://`) เพื่อปิดโดเมนอย่างราบรื่นหรือถ่ายสแนปช็อตขณะทำงาน มันไม่เคย bind-mount พาธ libvirt ใด ๆ จึงไม่ไปรบกวน VM Manager บนโฮสต์

BombVault คือชั้นการจัดการและ UI ไม่ใช่เอนจินจัดเก็บข้อมูล การเคลื่อนย้ายข้อมูลจริงทั้งหมดผ่าน restic

## เริ่มต้นอย่างรวดเร็ว

เพิ่งมาที่นี่? ไปที่ **[Getting started](getting-started.md)** เพื่อติดตั้ง BombVault บน Unraid ผ่าน Community Applications และรันการสำรองข้อมูลครั้งแรกของคุณ จากนั้นสำรวจ **[Features](features.md)** ฉบับเต็ม, ปรับแต่ง **[Configuration](configuration.md)** ของคุณ และตั้งค่า **[Off-site & recovery](offsite-recovery.md)**

การสำรองข้อมูลนอกสถานที่สามารถกระจายไปยังหลายปลายทางต่อโดเมนพร้อมกันได้ **แดชบอร์ดผู้รับ (receiver dashboard)** แบบอ่านอย่างเดียวจะตรวจสอบสำเนาเหล่านั้นบนเครื่องที่รับ และคุณสามารถนำการตั้งค่าทั้งหมดของคุณไปยังเครื่องใหม่ได้ด้วยการ์ด **ส่งออก / นำเข้าการตั้งค่า** ดู [Off-site & recovery](offsite-recovery.md) และ [Configuration](configuration.md#portable-settings-export-and-import)

**[แอป Android](android.md)** นำเซิร์ฟเวอร์ทุกเครื่องในกลุ่มของคุณมาไว้บนโทรศัพท์ พร้อมบันทึกกิจกรรมของทุกเครื่องในหน้าจอเดียว

## เครดิต {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** โดย [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) เป็นที่มาของแนวคิดตั้งต้นของ BombVault: การสำรองข้อมูลด้วยคลิกเดียวและการติดตั้ง Docker containers ใหม่โดยอัตโนมัติ BombVault เป็นการพัฒนาแยกต่างหากบน Go และ restic ที่ต่อยอดแนวคิดนี้ไปยัง VMs แฟลช และอื่น ๆ
- **[restic](https://restic.net/)** คือเอนจินสำรองข้อมูลที่รวดเร็ว ปลอดภัย และขจัดข้อมูลซ้ำ ซึ่ง BombVault ใช้ขับเคลื่อน
- **[rclone](https://rclone.org/)** ให้แบ็กเอนด์คลาวด์
- ไอคอนส่วนใหญ่บนปุ่มมาจากชุด Core Solid แบบฟรีของ **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [ต้นฉบับ](https://github.com/webalys-hq/streamline-vectors)) ส่วนที่เหลือมาจาก Font Awesome Free, Material Design Icons, Simple Icons และ Tabler Icons หรือวาดขึ้นสำหรับโปรเจกต์นี้

## สัญญาอนุญาต {#license}

Copyright (C) 2026 Junker der Provinz. BombVault เป็นซอฟต์แวร์เสรีภายใต้ **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)) คุณสามารถรัน ศึกษา แบ่งปัน และแก้ไขได้ หากคุณเผยแพร่ หรือรันเวอร์ชันที่แก้ไขแล้วเป็นบริการบนเครือข่าย คุณต้องเผยแพร่ซอร์สโค้ดของคุณภายใต้สัญญาอนุญาตเดียวกัน และคงประกาศลิขสิทธิ์และประกาศการระบุแหล่งที่มาเดิมไว้

ชื่อและแบรนด์ไม่ได้อยู่ภายใต้สัญญาอนุญาต AGPL ครอบคลุมเฉพาะซอร์สโค้ด: "BombVault" โลโก้ และแบรนด์ของมันยังสงวนสิทธิ์ไว้ ดังนั้น fork ต้องใช้ชื่อและแบรนด์ของตัวเอง และต้องไม่แสดงตัวว่าเป็น BombVault

## ลิงก์

- **โค้ดต้นฉบับ:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **กระทู้สนับสนุนของ Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **ปัญหา (Issues):** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "การควบคุมโฮสต์เทียบเท่า root"
    ผ่าน Docker socket, BombVault สามารถหยุด, ลบ และสร้าง containers ใหม่ รวมถึงอ่าน/เขียน appdata ได้ และสำหรับการสำรองข้อมูล VM มันจะล็อกอินเข้าโฮสต์ผ่าน SSH เพื่อรัน `virsh` ใครก็ตามที่เข้าถึงเว็บ UI ของมันได้ ก็มีสิทธิ์เทียบเท่า root บนโฮสต์ รัน BombVault บนเครือข่ายที่เชื่อถือได้และไม่เปิดเผยต่อภายนอกเท่านั้น และเปิดใช้งานด่านรหัสผ่านเสริม (การตั้งค่า, ความปลอดภัย) เมื่อมีการใช้การสำรองข้อมูลนอกสถานที่หรือแบบไม่เปลี่ยนแปลงได้ ดู [Configuration](configuration.md) สำหรับโมเดลความปลอดภัยฉบับเต็ม
