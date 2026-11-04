# أمثلة خطوة بخطوة

يشرح كل مثال من هذه الأمثلة مهمة حقيقية واحدة من أولها إلى آخرها، بأسماء وأرقام مملوءة. وهي موجودة في الصفحة التي تشرح الميزة نفسها، فتجد التفاصيل بجانبها مباشرة.

## البدء {#start}

- [أول نسخة احتياطية لحاوية، من البداية إلى النهاية](getting-started.md#example-first-backup): من التثبيت إلى حقيبة الاستعادة.

## التخزين وخارج الموقع {#storage}

- [نسخة خارج الموقع على Backblaze B2](offsite-recovery.md#example-s3): حاوية (bucket) ومفتاح لهذه الحاوية وأول نسخة. يعمل Cloudflare R2 بالطريقة نفسها.
- [خادم rest-server تشغّله بنفسك](offsite-recovery.md#example-rest-server): نسخة خارج الموقع ترفض الحذف، مع سطر `docker run` واختبار العبث.
- [ما الذي يحتفظ به «متوازن»](offsite-recovery.md#example-retention): الاحتفاظ على مدى 400 يوم من النسخ الليلية، نسخة بعد نسخة.
- [الحاويات تنتقل إلى NAS](offsite-recovery.md#example-own-folder): ينتقل المجلد الخاص بقسم إلى مكان آخر، وماذا يحدث للنسخ المأخوذة سابقًا.

## النسخ الاحتياطية والجداول {#backups}

- [ثلاث خطط تتكامل معًا](schedules.md#example): نسخة شاملة ليلًا، والأجهزة الافتراضية مرة في الأسبوع، وخطة خاصة لعنصر واحد كل 6 ساعات.
- [Plex ينتظر ما دام أحد يشاهد](backups.md#example-plex): نسخة احتياطية مجدولة لا تقطع فيلمًا.
- [جهاز افتراضي بلقطتين من Unraid](vm-backup-ssh-setup.md#example-unraid-snapshots): نسخة حية تحافظ على اللقطات، واستعادة بجانب الأصل.
- [نسخة طبق الأصل على جهاز Unraid ثانٍ](zfs-datasets.md#example-replica): نسخة ZFS يمكنك قراءتها في أي وقت، وثلاث طرق للحصول على ملف واحد منها.

## استعادة البيانات {#restore}

- [استعادة ملف واحد](restore.md#example-single-file): ملف واحد من نسخة الحاوية الليلية أمس.
- [ملف واحد من نسخة ZFS الاحتياطية](zfs-datasets.md#example-zfs-file): الأمر نفسه لمجموعة بيانات.
- [خادم جديد من حقيبة الاستعادة](offsite-recovery.md#example-emergency): خادم مات يعود على جهاز جديد، دون `APP_KEY` القديم في القالب.

## البقاء على اطلاع {#staying-informed}

- [Gotify على هاتفك](notifications.md#example-gotify): الأعطال ترنّ والنجاحات تصل بهدوء.
- [ntfy كـ webhook](notifications.md#example-ntfy): رسائل بلا حساب.

## عدة خوادم {#several-servers}

- [اقتران جهازي Unraid](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): خادمان يحتفظ كل منهما بنسخ الآخر، ولا يستطيع أي منهما حذف نسخ الآخر.
- [بطاقة وتنبيه في Home Assistant](api.md#home-assistant-example): BombVault على لوحتك، ورسالة عند فشل نسخة احتياطية.
