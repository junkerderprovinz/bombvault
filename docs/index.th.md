# BombVault

**ข้อมูล Unraid ของคุณ ถูกผนึกไว้ในห้องนิรภัย ปล่อยการสำรองข้อมูลลงไป จุดชนวนการกู้คืน**

BombVault คือเว็บแอปแบบ self-hosted ที่ออกแบบมาสำหรับ Unraid โดยเฉพาะ สำหรับ **การสำรองข้อมูลและการกู้คืนจากภัยพิบัติแบบเต็มรูปแบบ** ของ Docker containers และ KVM/libvirt VMs ของคุณ มันทำงานเป็น Docker container แบบ multi-arch เพียงตัวเดียว มอบเว็บ UI ธีมมืดที่ทันสมัย และจัดการวงจรทั้งหมด: สำรองข้อมูล, ตั้งตารางเวลา, ตรวจสอบ และกู้คืน

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

BombVault คือชั้นการจัดการและ UI ไม่ใช่เอนจินจัดเก็บข้อมูล การเคลื่อนย้ายข้อมูลจริงทั้งหมดผ่าน restic

## เริ่มต้นอย่างรวดเร็ว

เพิ่งมาที่นี่? ไปที่ **[Getting started](getting-started.md)** เพื่อติดตั้ง BombVault บน Unraid ผ่าน Community Applications และรันการสำรองข้อมูลครั้งแรกของคุณ จากนั้นสำรวจ **[Features](features.md)** ฉบับเต็ม, ปรับแต่ง **[Configuration](configuration.md)** ของคุณ และตั้งค่า **[Off-site & recovery](offsite-recovery.md)**

การสำรองข้อมูลนอกสถานที่สามารถกระจายไปยังหลายปลายทางต่อโดเมนพร้อมกันได้ **แดชบอร์ดผู้รับ (receiver dashboard)** แบบอ่านอย่างเดียวจะตรวจสอบสำเนาเหล่านั้นบนเครื่องที่รับ และคุณสามารถนำการตั้งค่าทั้งหมดของคุณไปยังเครื่องใหม่ได้ด้วยการ์ด **Export and import settings** ดู [Off-site & recovery](offsite-recovery.md) และ [Configuration](configuration.md#portable-settings-export-and-import)

## ลิงก์

- **โค้ดต้นฉบับ:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **กระทู้สนับสนุนของ Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **ปัญหา (Issues):** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "การควบคุมโฮสต์เทียบเท่า root"
    ผ่าน Docker socket, BombVault สามารถหยุด, ลบ และสร้าง containers ใหม่ รวมถึงอ่าน/เขียน appdata ได้ และสำหรับการสำรองข้อมูล VM มันจะล็อกอินเข้าโฮสต์ผ่าน SSH เพื่อรัน `virsh` ใครก็ตามที่เข้าถึงเว็บ UI ของมันได้ ก็มีสิทธิ์เทียบเท่า root บนโฮสต์ รัน BombVault บนเครือข่ายที่เชื่อถือได้และไม่เปิดเผยต่อภายนอกเท่านั้น และเปิดใช้งานด่านรหัสผ่านเสริม (Settings, Security) เมื่อมีการใช้การสำรองข้อมูลนอกสถานที่หรือแบบไม่เปลี่ยนแปลงได้ ดู [Configuration](configuration.md) สำหรับโมเดลความปลอดภัยฉบับเต็ม
