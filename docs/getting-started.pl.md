# Pierwsze kroki

Ta strona przeprowadzi Cię od świeżej maszyny Unraid do Twojej pierwszej kopii zapasowej.

## Wymagania

| Wymaganie | Uwagi |
|---|---|
| **Unraid 6.12+** | Wcześniejsze wersje nie są testowane. Unraid jest główną platformą, ale BombVault działa też na zwykłym hoście Dockera i na TrueNAS Scale (zobacz [Zwykły host Dockera](#generic-docker-host) i [TrueNAS SCALE](#truenas)). |
| **Lokalizacja repozytorium restic** | Ścieżka lokalna (zalecane: Twoja macierz lub cache), SMB, NFS lub dowolny backend rclone. |
| **Gniazdo Docker** | Montowane automatycznie przez szablon (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montowany w całości automatycznie przez szablon (`/boot` do `/host/boot`). Zasila kopię flash i pozwala przywróconemu kontenerowi pojawić się ponownie jako normalna, edytowalna aplikacja Unraid. |
| **Maszyny wirtualne KVM** (opcjonalnie) | Kopia VM komunikuje się z libvirt przez SSH, bez montowania libvirt. Skonfiguruj to w **Ustawienia, Integracje, SSH hosta** (zobacz [Konfiguracja](configuration.md)). |
| **Zbiory danych ZFS** (opcjonalnie) | To samo połączenie SSH co przy kopiach VM, `zfs` na hoście oraz Host Data zmapowane jako `/mnt` z trybem dostępu Read/Write - Slave, domyślnym w szablonie. Zobacz [Zbiory danych ZFS](zfs-datasets.md). |
| **Aplikacja na Androida** (opcjonalnie) | Android 10 lub nowszy, sparowana z serwerami w wersji 9.7.0 lub nowszej. Zobacz [Aplikacja na Androida](android.md). |

## Instalacja na Unraid

Najprostsza droga to **Community Applications**.

1. Otwórz zakładkę **Apps** w Unraid.
2. Wyszukaj **BombVault**.
3. Kliknij **Install**, ustaw wymagane zmienne (poniżej) i zastosuj.

!!! tip "Ręczna instalacja szablonu"
    Jeśli wolisz dodać szablon ręcznie:

    1. Przejdź do **Docker, Add Container, Template repositories** i dodaj:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Wyszukaj **BombVault** w Templates.
    3. Ustaw wymagane zmienne i kliknij **Apply**.

## Zwykły host Dockera {#generic-docker-host}

Nie Unraid? BombVault działa też jako zwykły kontener na dowolnym hoście Dockera. TrueNAS SCALE ma własne kroki w [TrueNAS SCALE](#truenas).

1. Pobierz z repozytorium gotowy do edycji plik [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml).
2. Ustaw `APP_KEY` (patrz niżej) i skieruj wolumen Host Data na swój prawdziwy katalog danych. Komentarze w pliku prowadzą przez jedno i drugie.
3. `docker compose up -d`, a następnie otwórz `https://<ip-hosta>:3443/`.

Czym różni się to od Unraida:

- **Brak sekcji Flash.** Nie ma pendrive'a rozruchowego do zabezpieczenia ani odtworzenia, więc sekcja Flash nie ma tu nic do roboty. Zamiast tego, gdy Host Data sięga do `/etc` hosta, **Dodaj** na stronie Kopie zapasowe proponuje **Konfiguracja systemu** jako podpowiedź folderu: początkowy zestaw folderów z `/etc`, który przeglądasz i poprawiasz przed zapisaniem.
- **Brak natywnych powiadomień Unraida.** Własne kanały powiadomień BombVaulta (webhook, alerty o nieudanej replikacji poza siedzibę i tak dalej) działają normalnie; pomijane jest tylko wysłanie do systemu powiadomień Unraida, bo takiego systemu tutaj nie ma.
- **Kopia maszyn wirtualnych jest opcjonalna i wymaga osobnego hosta libvirtd dostępnego przez SSH.** Zobacz zakomentowany blok w pliku compose. Zwykły host Dockera sam w sobie nie ma menedżera maszyn wirtualnych.
- **Brak widżetu na pulpicie.** BombVault Widget to wtyczka Unraida, więc ten krok również jest pomijany.
- **Odnajdywanie danych kontenera.** Bez konwencji `appdata` z Unraida folder danych kontenera jest odnajdywany na podstawie segmentów w `DATA_ROOT_SEGMENTS`, nazwanych woluminów Dockera, katalogu roboczego projektu Compose i etykiety `bombvault.data` (zobacz [Wykrywanie źródeł kopii zapasowej](configuration.md#backup-source-detection)). Nazwane woluminy i zestaw predefiniowany `/etc` sięgają tylko do ścieżek wewnątrz montowania Host Data, więc skieruj Host Data na wspólny katalog nadrzędny, który obejmuje też katalog danych Dockera.
- **`PLATFORM`.** Ustaw ją na `generic` albo `truenas`. Gdy nie jest ustawiona, BombVault rozpoznaje Unraid po jego własnym znaczniku na montowaniu flash, a wszystko inne traktuje jako zwykły host, i kroki tylko dla Unraida są pomijane, zamiast być próbowane i kończyć się błędem.

### TrueNAS SCALE {#truenas}

BombVault nie jest jeszcze w katalogu aplikacji TrueNAS, więc zainstaluj go jako własną aplikację: **Apps, Discover Apps**, a potem **Install via YAML** w menu obok **Custom App**. Poniższy YAML ustawia to samo co wpis katalogu w folderze `truenas-apps` repozytorium. Zastąp `tank` swoją pulą i uzupełnij `APP_KEY`, jak opisuje następna sekcja.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # Dla kopii VM:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Następnie otwórz `https://<truenas-ip>:30528/`. To, co wymieniono w [Czym różni się to od Unraida](#generic-docker-host), dotyczy także tego przypadku, a na TrueNAS:

- **Uprawnienia (capabilities).** BombVault potrzebuje sześciu powyższych. Bez `SETPCAP` kopie VM kończą się błędem.
- **Host Data** to ścieżka hosta na zbiorze danych, tutaj `/mnt/tank`, zamontowana pod tą samą ścieżką wewnątrz aplikacji. Dlatego `HOST_SOURCE_ROOT` i `HOST_MOUNT_ROOT` podają ją obie. `rslave` przekazuje montowania, które host wykonuje później, a tego potrzebują kopie zbiorów danych ZFS.
- **Dane w ixVolume nie są kopiowane.** Aplikacja, której pamięcią jest ixVolume, trzyma dane pod `/mnt/.ix-apps`, poza Host Data. BombVault zapisuje tylko definicję aplikacji, a przebieg podaje foldery, które pominął. Dla każdej aplikacji, której dane chcesz kopiować, wybierz pamięć **Host Path** na zbiorze danych pod Host Data.
- **Kopie VM i zbiorów danych ZFS** łączą się z TrueNAS przez SSH jako root. Włącz usługę SSH w **System, Services** i dodaj klucz publiczny z **Ustawienia, Integracje, SSH hosta** do użytkownika root w **Credentials, Users**. Kopie VM potrzebują też `LIBVIRT_URI` jak wyżej, bo libvirtd na TrueNAS nasłuchuje na własnym gnieździe. Gdy nie ustawiono ani `LIBVIRT_URI`, ani `LIBVIRT_HOST`, BombVault używa domyślnej bramy aplikacji jako hosta, bo `host.docker.internal` nie rozwiązuje się w aplikacji TrueNAS. [Poradnik kopii VM](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md#truenas-scale) opisuje szczegóły, także dla VM na zvolach.
- **Replika ZFS używa innego klucza.** Replika na drugim serwerze ZFS loguje się kluczem, który pokazuje okno repliki, a nie kluczem SSH hosta. Dodaj ten klucz na serwerze odbierającym.
- **Usunięta aplikacja wraca w dwóch krokach.** Najpierw zainstaluj ją ponownie w TrueNAS pod tą samą nazwą, co tworzy na nowo jej sieć, a potem przywróć jej dane w BombVault. Do tego czasu kontrola przed przywracaniem zgłasza brakującą sieć, a przywracanie odmawia, zanim cokolwiek zmieni.
- **Obok oryginału** tworzy zwykły kontener Dockera. Działa, ale ekran Apps w TrueNAS go nie wymienia.
- **Zrzuty baz danych** działają na montowaniach zbiorów danych. Import do folderu danych, który jest osobnym zbiorem danych, przenosi starą zawartość do folderu obok, więc nadrzędny zbiór danych potrzebuje na nią miejsca.

## Jedno wymagane ustawienie

Jedyną zmienną, którą musisz ustawić, jest `APP_KEY`, 32-bajtowy sekret w formacie hex (64 znaki hex) używany do wyprowadzenia hasła repozytorium restic.

Wygeneruj go na dowolnej maszynie:

```bash
openssl rand -hex 32
```

Wklej wynik do pola `APP_KEY` w szablonie (Unraid) albo do zmiennej środowiskowej `APP_KEY` w `docker-compose.yml` (zwykły host Docker).

!!! danger "Nie zgub swojego APP_KEY"
    Utrata `APP_KEY` sprawia, że zaszyfrowane kopie zapasowe stają się nieodzyskiwalne. Przechowuj go w bezpiecznym miejscu, oddzielnie od serwera. Gdy BombVault już działa, ustaw hasło logowania i pobierz **zestaw odzyskiwania** w **Ustawienia, Przywracanie** (zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md#recovery-kit)). Zawiera klucz, każde miejsce docelowe i polecenia restic.

Szablon montuje też za Ciebie gniazdo Docker, flash (`/boot`) oraz katalog główny **Host Data** (`/mnt`). Zarówno *źródła*, jak i *miejsca docelowe* kopii zapasowych znajdują się pod Host Data. Pełny opis zmiennych oraz konfigurację poza siedzibą znajdziesz w [Konfiguracja](configuration.md).

## Pierwsze uruchomienie

![Przegląd po pierwszej kopii: co wymaga uwagi, ochrona każdej sekcji, miejsca docelowe i historia.](assets/screenshots/dashboard.png)

*Przegląd po pierwszej kopii: co wymaga uwagi, ochrona każdej sekcji, miejsca docelowe i historia.*

1. Otwórz interfejs webowy pod adresem `https://<your-unraid-ip>:3443` (certyfikat samopodpisany od razu po instalacji). Nowa instalacja otwiera się na ekranie **Witamy w BombVault**, który wymienia to, co BombVault znalazł na serwerze.
2. Wybierz **Zaczynamy**. Przegląd pokaże wtedy kartę **Pierwsze kroki** z trzema krokami:
    1. **Wybierz sekcje**: włącz to, co BombVault ma kopiować (Containers, Maszyny wirtualne, Flash, Foldery, Zbiory danych ZFS, Autokopia). **Wybierz** otwiera listę elementów sekcji, na której **Wstrzymaj harmonogram** pomija jedną z nich. Sekcje włączone tutaj działają co noc, co zmienisz w **Ustawienia, Harmonogram**.
    2. **Dodaj miejsce docelowe**: dokąd trafiają kopie zapasowe. Na początek wystarczy folder lokalny. Najbezpieczniej mieć drugie miejsce docelowe poza domem, na przykład Storage Box albo Backblaze B2. Repozytoria lokalne domyślnie wynoszą `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` i są tworzone przy pierwszej kopii.
    3. **Pierwsza kopia zapasowa**: potem działa sama co noc.
3. Otwórz **Kopie zapasowe**, aby zobaczyć każdy element na jednej liście. **Utwórz kopię wszystkiego teraz** uruchamia przebieg po wszystkich włączonych sekcjach, a **Utwórz kopię teraz** na stronie elementu kopiuje tylko tę jedną.

Używasz już BombVault? **Przywróć z kopii zapasowej** na stronie powitalnej otwiera stronę Przywracanie, gdzie **Kreator awaryjny** przywraca ustawienia, kontenery, maszyny wirtualne i foldery utraconego serwera. Zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md#emergency).

!!! tip "Opcjonalnie: wybierz kolejność kopii zapasowych"
    Jeśli niektóre kontenery powinny być zawsze kopiowane przed innymi (na przykład baza danych przed aplikacją, która z niej korzysta), otwórz kartę **Kolejność** w **Ustawienia, Harmonogram** i przeciągnij je w wybraną sekwencję. Uruchomienia zaplanowane i pełna kopia będą jej przestrzegać; wszystko, co pozostawisz bez kolejności, jest kopiowane od najbardziej zaległych.

!!! note "Sprawdzenie integracji z hostem"
    Karta **Integracja z hostem** w przeglądzie sonduje każdy montaż i każde CLI (gniazdo Docker, libvirt, restic, qemu-img, rclone) i zgłasza wszelkie brakujące elementy, więc możesz potwierdzić, że kontener jest poprawnie połączony, zanim na nim polegasz. Czeka w **Dostosuj**, aż ją dodasz.

## Poruszanie się po aplikacji {#navigation}

![Ustawienia jako kafelki, obok nich karty wybranego kafelka. Każda zmiana jest zapisywana, gdy ją wprowadzasz.](assets/screenshots/settings.png)

*Ustawienia jako kafelki, obok nich karty wybranego kafelka. Każda zmiana jest zapisywana, gdy ją wprowadzasz.*

Pasek boczny ma pięć pozycji: **Przegląd**, **Anomalie** (gdy wykrywanie anomalii jest włączone), **Kopie zapasowe**, **Przywracanie** i, pod **Wyloguj się**, **Ustawienia**. Główna akcja strony unosi się w prawym dolnym rogu, a **Więcej** zawiera te rzadkie. Na telefonie pasek ma Przegląd, Kopie zapasowe, Przywracanie i Ustawienia, a **Więcej** resztę.

Strony i ustawienia zajmują całą szerokość okna. Okno trzyma swoje przyciski w stopce, a ten, który prowadzi dalej, jest po prawej. Na telefonie stopka z więcej niż trzema przyciskami zostawia drogę wyjścia i główną akcję, a resztę chowa pod **Więcej**.

- **Kopie zapasowe** wymieniają każdy kontener, maszynę wirtualną, zestaw folderów, nośnik flash, każdy zbiór danych ZFS i autokopię jako elementy. Kafelki nad listą wybierają rodzaj, a **Filtry**, **Sortuj** i **Szukaj** zawężają listę. Każdy element ma własną stronę z trzema kartami: **Kopie zapasowe**, **Ustawienia** i **Weryfikacja**. Przycisk wstecz po lewej stronie ikony elementu prowadzi do listy.
- **Przywracanie** to jedna strona dla wszystkiego: wybierz sekcję, wyszukaj element i wybierz kopię zapasową. Kreator awaryjny i inne źródła zaczynają się tutaj też.
- **Ustawienia** to czternaście kafelków: Ogólne, Wygląd, Harmonogram, Miejsca docelowe, Przywracanie, Powiadomienia, Weryfikacja, Bezpieczeństwo, Instancje, Parowanie, Integracje, Aplikacje, System i Info. Naciśnij `/` lub Ctrl+K, aby przeszukać je wszystkie, także po nazwach, które strony miały we wcześniejszych wersjach. Przeciągnij kafelki w swoją kolejność w **Wygląd**. Tam, gdzie kolumna kafelków obok strony ścisnęłaby ją, jak w oknie o szerokości około 1200 pikseli, kafelki stają się paskiem nad stroną, który przewija się na boki. Telefon pokazuje ten sam pasek.
- **Miejsca docelowe** i **Instancje** to kafelki ustawień z własnymi stronami. **Pokaż także na pasku bocznym** na ich kafelku przypina je do paska bocznego.

Nic nie ma przycisku Zapisz i nic się nie rozwija: rzadkie rzeczy mają własną kartę albo okno, a wyjaśnienia są w dymkach (i). Po aktualizacji notatki do wydania otwierają się same, a zielone kropki oznaczają, co się zmieniło. Kliknięcie ukrywa kropkę. **Nowości** na karcie Wersja w **Ustawienia, Info** liczy to, czego jeszcze nie widziałeś, i otwiera notatki do wydania oraz listę każdej zmiany z **Przejdź tam**, jako dwie karty jednego okna.

## Budowanie ze źródeł {#build-from-source}

BombVault to pojedyncza statyczna binarka Go, która serwuje JSON API i osadzony interfejs React. Najpierw zbuduj interfejs, potem uruchom binarkę:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

Zbudowanie interfejsu jest potrzebne także dla `go run`. Repozytorium śledzi pod `web/dist` tylko pusty znacznik, więc bez `npm --prefix web run build` binarka niczego nie osadza i odpowiada `500 SPA index not found`, czego należy się spodziewać. Dockera, libvirt i Unraida nie da się przetestować w CI, więc przed otwarciem pull requesta sprawdź montowania, restic i połączenie SSH do VM na prawdziwym hoście za pomocą karty **Integracja z hostem** w przeglądzie (lub `GET /api/spike`).

## Kolejne kroki

- Przejrzyj pełne **[Funkcje](features.md)**.
- Przenieś wszystkie serwery swojej grupy na telefon dzięki **[aplikacji na Androida](android.md)**.
- Dodaj jedno lub więcej miejsc docelowych poza domem (każda sekcja może kopiować do kilku naraz) i zapisz swój zestaw odzyskiwania, jak opisano w **[Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md)**.
- Klonujesz konfigurację lub przenosisz się na nową maszynę? Przenieś całą swoją konfigurację za pomocą karty **Eksport / import ustawień**. Zobacz [Konfiguracja](configuration.md#portable-settings-export-and-import).
- Napotkałeś problem? Zobacz **[Rozwiązywanie problemów](troubleshooting.md)**.
