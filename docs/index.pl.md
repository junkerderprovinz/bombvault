# BombVault

**Twoje dane z Unraid, zapieczętowane w skarbcu. Zrzuć kopię zapasową. Odpal przywracanie.**

BombVault to self-hostowana, natywna dla Unraid aplikacja webowa do **tworzenia kopii zapasowych i pełnego odzyskiwania po awarii** Twoich kontenerów Docker oraz maszyn wirtualnych KVM/libvirt. Działa jako pojedynczy, wieloarchitekturowy kontener Docker, daje Ci nowoczesny interfejs webowy dopasowujący się do preferencji jasnego/ciemnego motywu Twojego systemu, i obsługuje cały cykl życia: tworzenie kopii, harmonogramowanie, weryfikację i przywracanie.

Przywracanie jest automatyczne. Kontenery pojawiają się ponownie w zakładce Docker w Unraid dokładnie tak jak wcześniej, a maszyny wirtualne są na nowo definiowane w VM Manager z ponownie podpiętymi dyskami i pamięcią UEFI NVRAM. Bez ręcznej reinstalacji, bez rekonfiguracji, bez dramatów.

Napędzany przez [restic](https://restic.net), więc każda kopia zapasowa jest deduplikowana, przyrostowa i zawsze szyfrowana.

!!! note "Chroń swój APP_KEY"
    BombVault wyprowadza hasło repozytorium restic z 32-bajtowego sekretu o nazwie `APP_KEY`. Jego utrata sprawia, że zaszyfrowane kopie zapasowe stają się nieodzyskiwalne. Wygeneruj go poleceniem `openssl rand -hex 32` i przechowuj w bezpiecznym miejscu. Zobacz [Konfiguracja](configuration.md).

## Co chroni BombVault

| Domena | Co jest zapisywane |
|---|---|
| **Kontenery Docker** | Katalog appdata wraz z definicją kontenera (obraz, zmienne środowiskowe, porty, etykiety, wolumeny). |
| **Maszyny wirtualne KVM / libvirt** | Obraz(y) dysku VM, definicja XML oraz pamięć UEFI NVRAM, kopia tworzona przez SSH (bez montowania libvirt). |
| **Flash Unraid** | Cały nośnik USB flash (`/boot`): system operacyjny, licencja, konfiguracja macierzy, udziały, sieć i konfiguracja wtyczek. |
| **Konfiguracja aplikacji** | Własny katalog `/config` BombVault: baza ustawień, poświadczenia poza siedzibą oraz para kluczy SSH libvirt. |
| **Pliki i foldery** | Nazwane **zestawy plików**, dowolny folder na serwerze, każdy z opcjonalnymi wzorcami wykluczeń per zestaw. |
| **Zbiory danych ZFS** | Zbiór danych ze wszystkimi zbiorami pod nim, odczytany z jednej migawki ZFS i zapisany jak folder. Zobacz [Zbiory danych ZFS](zfs-datasets.md). |

## Przywracanie to gwiazda

Po skopiowaniu danych z powrotem z migawki restic BombVault odtwarza zapisaną definicję kontenera względem Docker API, więc kontener pojawia się ponownie w zakładce Docker w Unraid tak, jakby był tam od zawsze (ten sam obraz, te same ustawienia, te same mapowania portów). Maszyny wirtualne otrzymują na nowo zdefiniowany XML przez SSH oraz ponownie podpięte dyski i pamięć UEFI NVRAM, nawet po usunięciu VM.

Gdy kopia zapasowa zatrzymuje zależne kontenery, wracają one we właściwej kolejności: BombVault uruchamia je ponownie w kolejności `depends_on` z Compose i czeka, aż każdy zgłosi stan healthy, zanim wystartuje te, które od niego zależą, więc nic nie wyprzedzi bazy danych ani bramy, która jeszcze nie działa. Zobacz [Funkcje](features.md).

## Jak to działa

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

BombVault używa gniazda Docker, aby zatrzymywać kontenery przed kopią i odtwarzać je po przywróceniu. Dla maszyn wirtualnych uruchamia `virsh` na hoście przez SSH (`qemu+ssh://`), aby płynnie wyłączyć maszynę albo wykonać migawkę na żywo. Nigdy nie montuje (bind mount) żadnej ścieżki libvirt, więc nie może przeszkadzać VM Managerowi na hoście.

BombVault jest warstwą orkiestracji i interfejsu, a nie silnikiem magazynu. Cały faktyczny przepływ danych odbywa się przez restic.

## Szybki start

Jesteś tu nowy? Przejdź do **[Pierwsze kroki](getting-started.md)**, aby zainstalować BombVault na Unraid przez Community Applications i uruchomić swoją pierwszą kopię zapasową. Następnie poznaj pełne **[Funkcje](features.md)**, dostrój swoją **[Konfigurację](configuration.md)** i skonfiguruj **[Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md)**.

Kopie poza siedzibą mogą rozgałęziać się na kilka celów per domena naraz, tylko do odczytu **panel odbiornika** monitoruje te kopie na maszynie, która je otrzymuje, a całą swoją konfigurację możesz przenieść na nową maszynę za pomocą karty **Eksport / import ustawień**. Zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md) oraz [Konfiguracja](configuration.md#portable-settings-export-and-import).

**[Aplikacja na Androida](android.md)** przenosi na telefon wszystkie serwery Twojej grupy, z dziennikiem aktywności ich wszystkich na jednym ekranie.

## Podziękowania {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** autorstwa [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) dał BombVault punkt wyjścia: kopię zapasową jednym kliknięciem i automatyczną ponowną instalację kontenerów Docker. BombVault to osobna implementacja oparta na Go i restic, która przenosi ten pomysł na maszyny wirtualne, flash i nie tylko.
- **[restic](https://restic.net/)** to szybki, bezpieczny silnik kopii zapasowych z deduplikacją, którym steruje BombVault.
- **[rclone](https://rclone.org/)** zapewnia backendy chmurowe.
- Większość symboli na przyciskach pochodzi z darmowego zestawu Core Solid od **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [źródło](https://github.com/webalys-hq/streamline-vectors)). Pozostałe pochodzą z Font Awesome Free, Material Design Icons, Simple Icons i Tabler Icons albo zostały narysowane dla projektu.

## Licencja {#license}

Copyright (C) 2026 Junker der Provinz. BombVault jest wolnym oprogramowaniem na licencji **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Możesz go uruchamiać, badać, udostępniać i zmieniać. Jeśli go rozpowszechniasz albo uruchamiasz zmienioną wersję jako usługę sieciową, musisz opublikować swój kod źródłowy na tej samej licencji i zachować istniejące informacje o prawach autorskich i autorstwie.

Nazwa i identyfikacja wizualna nie są objęte licencją. AGPL obejmuje wyłącznie kod źródłowy: "BombVault", jego logo i jego identyfikacja wizualna pozostają zastrzeżone, więc fork musi używać własnej nazwy i identyfikacji wizualnej i nie może przedstawiać się jako BombVault.

## Odnośniki

- **Kod źródłowy:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Wątek wsparcia Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Zgłoszenia:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Kontrola nad hostem równoważna uprawnieniom root"
    Poprzez gniazdo Docker BombVault może zatrzymywać, usuwać i odtwarzać kontenery oraz odczytywać i zapisywać appdata, a w celu tworzenia kopii VM loguje się do hosta przez SSH, aby uruchomić `virsh`. Każdy, kto może dotrzeć do jego interfejsu webowego, ma faktycznie uprawnienia root na hoście. Uruchamiaj BombVault wyłącznie w zaufanej, nieudostępnianej na zewnątrz sieci i włącz opcjonalną bramę hasła (Ustawienia, Bezpieczeństwo), gdy tylko używasz kopii poza siedzibą lub kopii niezmiennych. Zobacz [Konfiguracja](configuration.md), aby poznać pełny model bezpieczeństwa.
