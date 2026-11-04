# BombVault

**Twoje dane z Unraid, zapieczętowane w skarbcu. Zrzuć kopię zapasową. Odpal przywracanie.**

![Przegląd: nagłówek stanu, to, co wymaga uwagi, ochrona każdej sekcji, miejsca docelowe i historia.](assets/screenshots/overview/home-dark.png#only-dark)
![Przegląd: nagłówek stanu, to, co wymaga uwagi, ochrona każdej sekcji, miejsca docelowe i historia.](assets/screenshots/overview/home-light.png#only-light)

BombVault to self-hostowana aplikacja webowa do **tworzenia kopii zapasowych i pełnego odzyskiwania po awarii** na Unraid i innych hostach Docker, takich jak TrueNAS SCALE, ZimaOS czy zwykły Linux. Kopiuje kontenery Docker, maszyny wirtualne KVM/libvirt, foldery, zbiory danych ZFS i dysk flash Unraid. Działa jako pojedynczy, wieloarchitekturowy kontener Docker, daje Ci interfejs webowy w ciemnym, jasnym lub systemowym motywie i obsługuje cały cykl życia: tworzenie kopii, harmonogramowanie, weryfikację i przywracanie.

Przywracanie jest automatyczne. Na Unraid kontenery pojawiają się ponownie w zakładce Docker dokładnie tak jak wcześniej, a maszyny wirtualne są na nowo definiowane w VM Manager z ponownie podpiętymi dyskami i pamięcią UEFI NVRAM. Bez ręcznej reinstalacji, bez rekonfiguracji, bez dramatów. Na TrueNAS kontener przywrócony obok oryginału działa jako zwykły kontener Docker, którego ekran Apps nie wymienia.

Napędzany przez [restic](https://restic.net), więc każda kopia zapasowa jest deduplikowana, przyrostowa i domyślnie szyfrowana.

!!! note "Chroń swój APP_KEY"
    BombVault wyprowadza hasło repozytorium restic z 32-bajtowego sekretu o nazwie `APP_KEY`. Jego utrata razem z zestawem odzyskiwania sprawia, że zaszyfrowane kopie zapasowe stają się nieodzyskiwalne. Wygeneruj go poleceniem `openssl rand -hex 32` i przechowuj w bezpiecznym miejscu. Zobacz [Konfiguracja](configuration.md).

## Co chroni BombVault {#what-bombvault-protects}

Każdy kontener, każda maszyna wirtualna, zestaw folderów, dysk flash, każdy zbiór danych ZFS i własne ustawienia BombVault to **element** z własną stroną. Elementy jednego rodzaju tworzą **sekcję**, którą włączasz w **Ustawienia, Ogólne**.

| Sekcja | Co jest zapisywane |
|---|---|
| **Containers** | Katalog appdata wraz z definicją kontenera (obraz, zmienne środowiskowe, porty, etykiety, wolumeny). |
| **Maszyny wirtualne** | Obraz(y) dysku VM, definicja XML oraz pamięć UEFI NVRAM, kopia tworzona przez SSH (bez montowania libvirt). |
| **Flash** | Cały dysk USB flash (`/boot`): system operacyjny, licencja, konfiguracja macierzy, udziały, sieć i konfiguracja wtyczek. |
| **Autokopia** | Własny katalog `/config` BombVault: baza ustawień, dane logowania do miejsc docelowych oraz para kluczy SSH libvirt. |
| **Foldery** | Dowolny folder na serwerze jako zestaw folderów, każdy z opcjonalnymi wzorcami wykluczeń. |
| **Zbiory danych ZFS** | Zbiór danych ze wszystkimi zbiorami pod nim, odczytany z jednej migawki ZFS i zapisany jak folder. Zobacz [Zbiory danych ZFS](zfs-datasets.md). |

## Przywracanie to gwiazda {#restore-is-the-star}

Po skopiowaniu danych z powrotem z migawki restic BombVault odtwarza zapisaną definicję kontenera względem Docker API, więc kontener pojawia się ponownie w zakładce Docker w Unraid tak, jakby był tam od zawsze (ten sam obraz, te same ustawienia, te same mapowania portów). Maszyny wirtualne dostają na nowo zdefiniowany XML przez SSH oraz ponownie podpięte dyski i pamięć UEFI NVRAM, nawet po usunięciu maszyny.

Jedno okno przywraca każdy rodzaj. Domyślnie wybiera **Obok oryginału**, co przywraca kopię zapasową jako kopię i zostawia oryginał bez zmian. **Zastąp oryginał** najpierw tworzy kopię zapasową bieżącego stanu, więc przywrócenie można cofnąć. Folder lub nowy zbiór danych ZFS to pozostałe cele.

Gdy kopia zapasowa zatrzymuje zależne kontenery, wracają one we właściwej kolejności: BombVault uruchamia je ponownie w kolejności `depends_on` z Compose i czeka, aż każdy zgłosi stan healthy, zanim wystartuje te, które od niego zależą, więc nic nie wyprzedzi bazy danych ani bramy, która jeszcze nie działa. Zobacz [Przywracanie](restore.md) i [Polecenia i inne kontenery](backups.md#commands).

## Jak to działa {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault używa gniazda Docker, aby zatrzymywać kontenery przed kopią i odtwarzać je po przywróceniu. Dla maszyn wirtualnych uruchamia `virsh` na hoście przez SSH (`qemu+ssh://`), aby łagodnie wyłączyć maszynę albo wykonać migawkę na żywo. Nigdy nie montuje (bind mount) żadnej ścieżki libvirt, więc nie może przeszkadzać VM Managerowi na hoście.

BombVault jest warstwą orkiestracji i interfejsu, a nie silnikiem magazynu. Cały faktyczny przepływ danych odbywa się przez restic.

## Szybki start {#quick-start}

Zaczynasz? Przejdź do **[Pierwsze kroki](getting-started.md)**, aby zainstalować BombVault i uruchomić pierwszą kopię zapasową, a potem skonfiguruj **[Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md)**. **[Przykłady krok po kroku](guides.md)** prowadzą przez typowe zadania od początku do końca, strona **[Ustawienia](settings.md)** wymienia każdy kafelek, a **[Funkcje](features.md)** to krótki katalog wszystkiego, co robi BombVault.

Każda sekcja może kopiować naraz do kilku miejsc docelowych, sparowany BombVault może trzymać Twoje kopie poza siedzibą na serwerze, który odmawia usuwania, a **Kreator awaryjny** przywraca utracony serwer z zestawu odzyskiwania, przez Twoją grupę lub z wpisanym kluczem. Aby przenieść konfigurację na nową maszynę, użyj karty **Eksport / import ustawień**. Zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md) oraz [Konfiguracja](configuration.md#portable-settings-export-and-import).

**[Aplikacja na Androida](android.md)** przenosi na telefon wszystkie serwery Twojej grupy, z dziennikiem aktywności ich wszystkich na jednym ekranie.

Dokumentacja jest w 26 językach, a interfejs w 42. Strony, które nie mają jeszcze tłumaczenia, pokazują się po angielsku.

## Słowa używane tutaj {#glossary}

Kilka słów ma w BombVault i na tych stronach stałe znaczenie.

| Słowo | Znaczenie |
|---|---|
| **Element** | Jedna rzecz, którą kopiuje BombVault: kontener, maszyna wirtualna, zestaw folderów, dysk flash, zbiór danych ZFS ze zbiorami pod nim albo własne ustawienia. Każdy ma własną stronę. |
| **Sekcja** | Wszystkie elementy jednego rodzaju, na przykład Containers. Sekcje włączasz w **Ustawienia, Ogólne**. |
| **Miejsce docelowe** | Miejsce, dokąd trafiają kopie: folder na tym serwerze, udział, własny serwer albo usługa w chmurze. Zobacz [Miejsca docelowe](offsite-recovery.md#destinations). |
| **Lokalne** | Miejsce docelowe na tym serwerze, które przechowuje foldery sekcji. |
| **Własny folder** | Repozytorium, do którego sekcja zapisuje kopie najpierw. Domyślnie jest na tym serwerze, a może leżeć w miejscu docelowym. Zobacz [Własny folder sekcji](offsite-recovery.md#own-folder). |
| **Kopia (copy)** | Kopia zapasowa skopiowana z własnego folderu do innego miejsca docelowego poleceniem `restic copy`. |
| **Repozytorium bezpośrednie** | Repozytorium w miejscu poza siedzibą, do którego elementy kopiują wprost, bez lokalnej kopii pośrodku. Zobacz [Repozytoria bezpośrednie](offsite-recovery.md#direct-repositories). |
| **Dane logowania** | Zapisany klucz lub hasło do usługi S3 albo rest-servera. Jedno z nich jest domyślne. Zobacz [Dane logowania](offsite-recovery.md#sign-ins). |
| **Ustawienie** | Które miejsca docelowe dostają kopie elementu, ustawiane na elemencie albo jako domyślne dla jego sekcji. Zobacz [Ustawienie dla elementu](offsite-recovery.md#placement). |
| **Replika** | Kopia ZFS co do bloku na drugim serwerze ZFS, zrobiona przez `zfs send`. Stoi obok kopii zapasowej i jej nie zastępuje. Zobacz [Replika](zfs-datasets.md#replica). |
| **Rola** | To, co jedna instancja Twojej grupy robi dla drugiej: Odbiornik, Pobierający, Serwer ZFS albo Przechowawca klucza. Zobacz [Instancje i role](offsite-recovery.md#roles). |

## Podziękowania {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** autorstwa [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) dał BombVault punkt wyjścia: kopię zapasową jednym kliknięciem i automatyczną ponowną instalację kontenerów Docker. BombVault to osobna implementacja oparta na Go i restic, która przenosi ten pomysł na maszyny wirtualne, flash i nie tylko.
- **[restic](https://restic.net/)** to szybki, bezpieczny silnik kopii zapasowych z deduplikacją, którym steruje BombVault.
- **[rclone](https://rclone.org/)** zapewnia backendy chmurowe.
- Większość symboli na przyciskach pochodzi z darmowego zestawu Core Solid od **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [źródło](https://github.com/webalys-hq/streamline-vectors)). Pozostałe pochodzą z Font Awesome Free, Material Design Icons, Simple Icons i Tabler Icons albo zostały narysowane dla projektu.

## Licencja {#license}

Copyright (C) 2026 Junker der Provinz. BombVault jest wolnym oprogramowaniem na licencji **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Możesz go uruchamiać, badać, udostępniać i zmieniać. Jeśli go rozpowszechniasz albo uruchamiasz zmienioną wersję jako usługę sieciową, musisz opublikować swój kod źródłowy na tej samej licencji i zachować istniejące informacje o prawach autorskich i autorstwie.

Nazwa i identyfikacja wizualna nie są objęte licencją. AGPL obejmuje wyłącznie kod źródłowy: „BombVault”, jego logo i jego identyfikacja wizualna pozostają zastrzeżone, więc fork musi używać własnej nazwy i identyfikacji wizualnej i nie może przedstawiać się jako BombVault.

## Odnośniki {#links}

- **Kod źródłowy:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Wątek wsparcia Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Zgłoszenia:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Kontrola nad hostem równoważna uprawnieniom root"
    Poprzez gniazdo Docker BombVault może zatrzymywać, usuwać i odtwarzać kontenery oraz odczytywać i zapisywać appdata, a przy kopii maszyn wirtualnych loguje się do hosta przez SSH, aby uruchomić `virsh`. Każdy, kto może dotrzeć do jego interfejsu webowego, ma faktycznie uprawnienia root na hoście. Uruchamiaj BombVault wyłącznie w zaufanej, nieudostępnianej na zewnątrz sieci i ustaw hasło logowania w **Ustawienia, Bezpieczeństwo**. Zestaw odzyskiwania, depozyty kluczy i usuwanie kopii zapasowych w miejscu docelowym go wymagają. Zobacz [Konfiguracja](configuration.md#security-model), aby poznać pełny model bezpieczeństwa.
