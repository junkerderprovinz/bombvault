# Przykłady krok po kroku

Każdy z nich prowadzi przez jedno prawdziwe zadanie od początku do końca, z wpisanymi nazwami i liczbami. Stoją na stronie, która objaśnia daną funkcję, więc szczegóły są tuż obok.

## Pierwsze kroki {#start}

- [Pierwsza kopia kontenera od początku do końca](getting-started.md#example-first-backup): od instalacji do zestawu odzyskiwania.

## Magazyn i poza siedzibą {#storage}

- [Kopia poza siedzibą w Backblaze B2](offsite-recovery.md#example-s3): koszyk, klucz do tego koszyka i pierwsza kopia. Cloudflare R2 działa tak samo.
- [Rest-server, który prowadzisz sam](offsite-recovery.md#example-rest-server): kopia poza siedzibą, która odmawia usuwania, z linią `docker run` i testem manipulacji.
- [Co zachowuje Zrównoważone](offsite-recovery.md#example-retention): przechowywanie na przestrzeni dwóch lat nocnych kopii, kopia po kopii.
- [Kontenery przenoszą się na NAS](offsite-recovery.md#example-own-folder): własny folder sekcji zmienia miejsce i co dzieje się z kopiami już wykonanymi.

## Kopie i harmonogramy {#backups}

- [Trzy plany, które do siebie pasują](schedules.md#example): pełna kopia w nocy, maszyny wirtualne raz w tygodniu i własny plan dla jednego elementu co 6 godzin.
- [Plex czeka, gdy ktoś ogląda](backups.md#example-plex): zaplanowana kopia, która nie przerywa filmu.
- [Maszyna wirtualna z dwiema migawkami Unraid](vm-backup-ssh-setup.md#example-unraid-snapshots): kopia na żywo, która zachowuje migawki, i przywracanie obok oryginału.
- [Replika na drugi serwer Unraid](zfs-datasets.md#example-replica): kopia ZFS, którą możesz czytać w każdej chwili, i trzy sposoby na wyciągnięcie z niej jednego pliku.

## Odzyskiwanie danych {#restore}

- [Odzyskaj jeden plik](restore.md#example-single-file): pojedynczy plik z nocnej kopii kontenera.
- [Jeden plik z kopii ZFS](zfs-datasets.md#example-zfs-file): to samo dla zbioru danych.
- [Nowy serwer z zestawu odzyskiwania](offsite-recovery.md#example-emergency): martwy serwer wraca na nowej maszynie, bez dawnego `APP_KEY` w szablonie.

## Bądź na bieżąco {#staying-informed}

- [Gotify w telefonie](notifications.md#example-gotify): błędy dzwonią, a sukcesy przychodzą po cichu.
- [ntfy jako webhook](notifications.md#example-ntfy): wiadomości bez konta.

## Kilka serwerów {#several-servers}

- [Parowanie dwóch maszyn Unraid](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): dwa serwery, które trzymają nawzajem swoje kopie, a żaden nie może usunąć kopii drugiego.
- [Karta i alert w Home Assistant](api.md#home-assistant-example): BombVault na pulpicie z wiadomością, gdy kopia się nie uda.
