# BombVault

**بياناتك على Unraid، مختومة داخل خزنة. أسقِط نسخة احتياطية. فجّر استعادة.**

BombVault هو تطبيق ويب ذاتي الاستضافة ومصمَّم أصلاً لـ Unraid من أجل **النسخ الاحتياطي والتعافي الكامل من الكوارث** للـ Docker containers ولأجهزة KVM/libvirt الافتراضية. يعمل كحاوية Docker واحدة متعددة المعماريات، ويمنحك واجهة ويب عصرية تتبع تفضيل نظامك للوضع الفاتح/الداكن، ويدير دورة الحياة بأكملها: النسخ الاحتياطي والجدولة والتحقق والاستعادة.

الاستعادة تلقائية. تظهر الـ containers من جديد في تبويب Docker على Unraid تماماً كما كانت، ويُعاد تعريف الـ VMs في مدير الأجهزة الافتراضية مع إعادة ربط أقراصها وذاكرة UEFI NVRAM. بدون إعادة تثبيت يدوي، وبدون إعادة إعداد، وبدون متاعب.

مدعوم بـ [restic](https://restic.net)، لذا فإن كل نسخة احتياطية مُزالة التكرار وتزايدية ومشفَّرة دائماً.

!!! note "احتفظ بـ APP_KEY في أمان"
    يشتق BombVault كلمة مرور مستودع restic من سر بحجم 32 بايت باسم `APP_KEY`. فقدانه يجعل النسخ الاحتياطية المشفّرة غير قابلة للاسترداد. أنشئ واحداً بـ `openssl rand -hex 32` واحفظه في مكان آمن. راجع [الإعدادات](configuration.md).

## ما الذي يحميه BombVault

| النطاق | ما الذي يُحفَظ |
|---|---|
| **Docker containers** | مجلد appdata إضافة إلى تعريف الحاوية (الصورة، متغيرات البيئة، المنافذ، التسميات، الوحدات). |
| **KVM / libvirt VMs** | صورة (صور) قرص الـ VM، وتعريف XML، وذاكرة UEFI NVRAM، منسوخة احتياطياً عبر SSH (دون تركيب libvirt). |
| **Unraid flash** | فلاش USB بالكامل (`/boot`): نظام التشغيل، الترخيص، إعداد المصفوفة، المشاركات، وإعداد الشبكة والإضافات. |
| **إعدادات التطبيق** | مجلد `/config` الخاص بـ BombVault: قاعدة بيانات إعداداته، وبيانات الاعتماد خارج الموقع، وزوج مفاتيح libvirt SSH. |
| **الملفات والمجلدات** | **مجموعات ملفات** مسمّاة، أي مجلد على الخادم، ولكلٍّ منها أنماط استبعاد اختيارية خاصة بكل مجموعة. |
| **مجموعات بيانات ZFS** | مجموعة بيانات مع كل ما تحتها، تُقرأ من لقطة ZFS واحدة وتُحفظ مثل مجلد. راجع [مجموعات بيانات ZFS](zfs-datasets.md). |

## الاستعادة هي النجم

بعد نسخ البيانات مرة أخرى من لقطة restic، يعيد BombVault تطبيق تعريف الحاوية المحفوظ على واجهة Docker API، فتظهر الحاوية من جديد في تبويب Docker على Unraid كأنها كانت هناك دائماً (نفس الصورة، نفس الإعدادات، نفس تعيينات المنافذ). أما الـ VMs فيُعاد تعريف XML الخاص بها عبر SSH ويُعاد ربط أقراصها وذاكرة UEFI NVRAM، حتى بعد حذف الـ VM.

عندما توقف نسخة احتياطية حاويات معتمِدة، تعود بالترتيب الصحيح: يعيد BombVault تشغيلها وفق ترتيب `depends_on` في Compose وينتظر أن يبلّغ كلٌّ منها عن حالته الصحية قبل تشغيل تلك التي تعتمد عليه، فلا يسبق شيء قاعدة بيانات أو بوابة لم تعمل بعد. راجع [الميزات](features.md).

## كيف يعمل

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

يستخدم BombVault مقبس Docker لإيقاف الحاويات قبل النسخ الاحتياطي ولإعادة إنشائها بعد الاستعادة. أما الأجهزة الافتراضية فيشغّل لها `virsh` على المضيف عبر SSH (`qemu+ssh://`) ليوقف الجهاز بشكل آمن أو يأخذ لقطة حية. ولا يركّب أي مسار libvirt أبدًا، فلا يمكنه أن يعترض عمل مدير الأجهزة الافتراضية على المضيف.

BombVault هو طبقة التنسيق والواجهة، وليس محرك التخزين. تمر كل عمليات نقل البيانات الفعلية عبر restic.

## بداية سريعة

جديد هنا؟ توجّه إلى **[البدء](getting-started.md)** لتثبيت BombVault على Unraid عبر Community Applications وتشغيل أول نسخة احتياطية لك. ثم استكشف كامل **[الميزات](features.md)**، واضبط **[الإعدادات](configuration.md)**، وأعدّ **[النسخ خارج الموقع والتعافي](offsite-recovery.md)**.

يمكن للنسخ خارج الموقع أن يتوزّع على عدة أهداف لكل نطاق في آن واحد، وتراقب **لوحة تحكم المُستقبِل** للقراءة فقط تلك النسخ على الجهاز الذي يستقبلها، ويمكنك نقل إعداداتك بالكامل إلى جهاز جديد ببطاقة **تصدير / استيراد الإعدادات**. راجع [النسخ خارج الموقع والتعافي](offsite-recovery.md) و[الإعدادات](configuration.md#portable-settings-export-and-import).

يجمع **[تطبيق Android](android.md)** كل خوادم مجموعتك على هاتفك، مع سجل النشاط لها جميعًا في شاشة واحدة.

## شكر وتقدير {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** من [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) منح BombVault فكرته الأولى: نسخ حاويات Docker احتياطيًا بنقرة واحدة وإعادة تثبيتها تلقائيًا. BombVault تنفيذ مستقل مبني على Go وrestic، يمد الفكرة إلى الأجهزة الافتراضية والفلاش وغيرها.
- **[restic](https://restic.net/)** هو محرك النسخ الاحتياطي السريع والآمن المُزيل للتكرار الذي يقوده BombVault.
- **[rclone](https://rclone.org/)** يوفّر الخلفيات السحابية.
- معظم الرموز على الأزرار من مجموعة Core Solid المجانية من **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)، [المصدر](https://github.com/webalys-hq/streamline-vectors)). والبقية من Font Awesome Free وMaterial Design Icons وSimple Icons وTabler Icons، أو رُسمت خصيصًا للمشروع.

## الترخيص {#license}

Copyright (C) 2026 Junker der Provinz. BombVault برنامج حر مرخَّص بموجب **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). يحق لك تشغيله ودراسته ومشاركته وتعديله. إذا وزّعته، أو شغّلت نسخة معدّلة منه كخدمة شبكية، فيجب عليك نشر الكود المصدري الخاص بك بموجب الترخيص نفسه والإبقاء على إشعارات حقوق النشر والإسناد القائمة.

الاسم والهوية التجارية غير مرخَّصين. يغطي ترخيص AGPL الكود المصدري فقط: "BombVault" وشعاره وهويته التجارية تبقى حقوقها محفوظة، لذا يجب على أي نسخة متفرّعة (fork) أن يستخدم اسمًا وهوية تجارية خاصين به، ولا يجوز له أن يقدّم نفسه على أنه BombVault.

## روابط

- **الكود المصدري:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **موضوع دعم Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **المشكلات:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "تحكّم بالمضيف يعادل صلاحيات root"
    عبر مقبس Docker يستطيع BombVault إيقاف الحاويات وإزالتها وإعادة إنشائها وقراءة/كتابة appdata، ولأجل نسخ الـ VM احتياطياً يسجّل الدخول إلى المضيف عبر SSH لتشغيل `virsh`. أي شخص يستطيع الوصول إلى واجهة الويب لديه فعلياً صلاحيات root على المضيف. شغّل BombVault فقط على شبكة موثوقة وغير معرّضة، وفعّل بوابة كلمة المرور الاختيارية (الإعدادات، الأمان) بمجرد استخدام النسخ خارج الموقع أو النسخ غير القابلة للتغيير. راجع [الإعدادات](configuration.md) لنموذج الأمان الكامل.
