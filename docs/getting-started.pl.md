# Pierwsze kroki

Ta strona przeprowadzi Cię od świeżej maszyny Unraid do Twojej pierwszej kopii zapasowej.

## Wymagania

| Wymaganie | Uwagi |
|---|---|
| **Unraid 6.12+** | Wcześniejsze wersje nie są testowane. Unraid jest główną platformą, ale BombVault działa też na zwykłym hoście Dockera i na TrueNAS Scale (zobacz [Zwykły host Dockera](#generic-docker-host)). |
| **Lokalizacja repozytorium restic** | Ścieżka lokalna (zalecane: Twoja macierz lub cache), SMB, NFS lub dowolny backend rclone. |
| **Gniazdo Docker** | Montowane automatycznie przez szablon (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montowany w całości automatycznie przez szablon (`/boot` do `/host/boot`). Zasila kopię flash i pozwala przywróconemu kontenerowi pojawić się ponownie jako normalna, edytowalna aplikacja Unraid. |
| **Maszyny wirtualne KVM** (opcjonalnie) | Kopia VM komunikuje się z libvirt przez SSH, bez montowania libvirt. Skonfiguruj to w Ustawieniach (zobacz [Konfiguracja](configuration.md)). |
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

Nie Unraid? BombVault działa też jako zwykły kontener na dowolnym hoście Dockera (na tym opiera się również obsługa kontenerów na TrueNAS Scale, zanim pojawi się tam własny wpis w katalogu aplikacji).

1. Pobierz z repozytorium gotowy do edycji plik [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml).
2. Ustaw `APP_KEY` (patrz niżej) i skieruj wolumen Host Data na swój prawdziwy katalog danych: komentarze w pliku prowadzą przez jedno i drugie.
3. `docker compose up -d`, a następnie otwórz `https://<ip-hosta>:3443/`.

Czym różni się to od Unraida:

- **Brak domeny flash/USB.** Nie ma pendrive'a rozruchowego do zabezpieczenia ani odtworzenia, więc domena Flash w ustawieniach nie ma tu nic do roboty. Zamiast tego domena Foldery proponuje jednym kliknięciem **Dodaj predefiniowany zestaw: konfiguracja systemu hosta** (początkowy zestaw plików `/etc`, który przeglądasz i poprawiasz przed zapisaniem), jako praktyczny ogólny odpowiednik.
- **Brak natywnych powiadomień Unraida.** Własne kanały powiadomień BombVaulta (webhook, alerty o nieudanej replikacji poza siedzibę i tak dalej) działają normalnie; pomijane jest tylko wysłanie do systemu powiadomień Unraida, bo takiego systemu tutaj nie ma.
- **Kopia maszyn wirtualnych jest opcjonalna i wymaga osobnego hosta libvirtd dostępnego przez SSH.** Zobacz zakomentowany blok w pliku compose. Zwykły host Dockera sam w sobie nie ma menedżera maszyn wirtualnych.
- **Brak widżetu na pulpicie.** BombVault Widget to wtyczka Unraida, więc ten krok również jest pomijany.
- **Odnajdywanie danych kontenera.** Bez konwencji `appdata` z Unraida folder danych kontenera jest odnajdywany na podstawie segmentów w `DATA_ROOT_SEGMENTS`, nazwanych woluminów Dockera, katalogu roboczego projektu Compose i etykiety `bombvault.data` (zobacz [Wykrywanie źródeł kopii zapasowej](configuration.md#backup-source-detection)). Nazwane woluminy i zestaw predefiniowany `/etc` sięgają tylko do ścieżek wewnątrz montowania Host Data, więc skieruj Host Data na wspólny katalog nadrzędny, który obejmuje też katalog danych Dockera.
- **`PLATFORM`.** Ustaw ją na `generic` albo `truenas`. Gdy nie jest ustawiona, BombVault rozpoznaje Unraid po jego własnym znaczniku na montowaniu flash, a wszystko inne traktuje jako zwykły host, i kroki tylko dla Unraida są pomijane, zamiast być próbowane i kończyć się błędem.

**TrueNAS Scale** idzie tą samą drogą przez compose; wpis do katalogu jest przygotowany w repozytorium, ale jeszcze go nie zgłoszono. Kopia VM wymaga tam `LIBVIRT_URI`, bo libvirtd na TrueNAS nasłuchuje na własnym gnieździe (`/run/truenas_libvirt/libvirt-sock`), którego trzy zmienne `LIBVIRT_*` nie potrafią wyrazić (zobacz [Konfiguracja](configuration.md)). Na ile to sprawdzono: kopię zvola wykonano na prawdziwej maszynie z TrueNAS Scale, na zvolu podłączonym do działającej VM, a `zfs snapshot`, `zfs send`, restic i `zfs receive` przeniosły go tam i z powrotem bajt w bajt. Pełnego przywracania sterowanego przez sam BombVault nie uruchomiono jeszcze na sprzęcie TrueNAS, a ten zvol był rzadki (sparse), więc przepustowość przy wielu gigabajtach nie jest przetestowana. Zanim zaczniesz na tym polegać, przetestuj tam przywracanie.

## Jedno wymagane ustawienie

Jedyną zmienną, którą musisz ustawić, jest `APP_KEY`, 32-bajtowy sekret w formacie hex (64 znaki hex) używany do wyprowadzenia hasła repozytorium restic.

Wygeneruj go na dowolnej maszynie:

```bash
openssl rand -hex 32
```

Wklej wynik do pola `APP_KEY` w szablonie (Unraid) albo do zmiennej środowiskowej `APP_KEY` w `docker-compose.yml` (zwykły host Docker).

!!! danger "Nie zgub swojego APP_KEY"
    Utrata `APP_KEY` sprawia, że zaszyfrowane kopie zapasowe stają się nieodzyskiwalne. Przechowuj go w bezpiecznym miejscu, oddzielnie od serwera. Gdy BombVault już działa, użyj jego funkcji **zestaw odzyskiwania klucza szyfrowania** dostępnej za jednym kliknięciem (zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md)), aby zapisać pełny pakiet odzyskiwania.

Szablon montuje też za Ciebie gniazdo Docker, flash (`/boot`) oraz katalog główny **Host Data** (`/mnt`). Zarówno *źródła*, jak i *cele* kopii zapasowych znajdują się pod Host Data. Pełny opis zmiennych oraz konfigurację poza siedzibą znajdziesz w [Konfiguracja](configuration.md).

## Pierwsze uruchomienie

![Pulpit po pierwszej kopii: co jest chronione, co uruchomi się dalej i dziennik na żywo.](assets/screenshots/dashboard.png)

*Pulpit po pierwszej kopii: co jest chronione, co uruchomi się dalej i dziennik na żywo.*

1. Otwórz interfejs webowy pod adresem `https://<your-unraid-ip>:3443` (certyfikat samopodpisany od razu po instalacji).
2. W **Ustawieniach** włącz domeny kopii zapasowych, których chcesz używać (Containers, Maszyny wirtualne, Flash, Autokopia, Foldery, Zbiory danych ZFS) i wybierz kolor akcentu.
3. W zakładce **Kontenery** wybierz kontener i kliknij **Utwórz kopię teraz**, aby stworzyć swój pierwszy punkt przywracania. Ścieżki repozytoriów domyślnie wynoszą `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` i są tworzone przy pierwszej kopii.
4. Skonfiguruj harmonogramowanie w **Ustawienia, Harmonogramy**. Dostępna jest funkcja *Uwzględnij wszystkie w harmonogramie* za jednym kliknięciem dla kontenerów i VM.

!!! tip "Opcjonalnie: wybierz kolejność kopii zapasowych"
    Jeśli niektóre kontenery powinny być zawsze kopiowane przed innymi (na przykład baza danych przed aplikacją, która z niej korzysta), otwórz panel **Kolejność kopii zapasowych** na stronie Containers i przeciągnij je w wybraną sekwencję. Uruchomienia zaplanowane i wielokrotnego wyboru będą jej przestrzegać; wszystko, co pozostawisz bez kolejności, jest kopiowane od najbardziej zaległych, jak poprzednio.

!!! note "Sprawdzenie integracji z hostem"
    Otwórz `/spike` w interfejsie webowym po uruchomieniu kontenera. Sonduje ono każdy montaż i każde CLI (gniazdo Docker, libvirt, restic, qemu-img, rclone) i zgłasza wszelkie brakujące elementy, więc możesz potwierdzić, że kontener jest poprawnie połączony, zanim na nim polegasz.

## Prosty vs Zaawansowany

![Ustawienia nie mają przycisku Zapisz: każda zmiana jest zapisywana od razu.](assets/screenshots/settings.png)

*Ustawienia nie mają przycisku Zapisz: każda zmiana jest zapisywana od razu.*

Domyślnie interfejs pokazuje tylko rzeczy podstawowe (tworzenie kopii, przywracanie, harmonogram). Użyj przełącznika **Widok prosty / Widok zaawansowany** w panelu bocznym, aby odsłonić kontrolki dla ekspertów: przechowywanie, kopię poza siedzibą, haki pre/post, przywracanie na poziomie plików, powiadomienia, metryki Prometheus oraz narzędzia integralności/konserwacji. To preferencja per przeglądarka, domyślnie wyłączona, więc nowicjusze dostają czysty interfejs, a użytkownicy zaawansowani mają wszystko.

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

Zbudowanie interfejsu jest potrzebne także dla `go run`. Repozytorium śledzi pod `web/dist` tylko pusty znacznik, więc bez `npm --prefix web run build` binarka niczego nie osadza i odpowiada `500 SPA index not found`, czego należy się spodziewać. Dockera, libvirt i Unraida nie da się przetestować w CI, więc przed otwarciem pull requesta sprawdź montowania, restic i połączenie SSH do VM na prawdziwym hoście za pomocą Sprawdzenia integracji z hostem (`/spike`).

## Kolejne kroki

- Przejrzyj pełne **[Funkcje](features.md)**.
- Przenieś wszystkie serwery swojej grupy na telefon dzięki **[aplikacji na Androida](android.md)**.
- Dodaj jedną lub więcej replik **[Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md)** (każda domena może wysyłać do kilku celów naraz) i zapisz swój zestaw odzyskiwania.
- Klonujesz konfigurację lub przenosisz się na nową maszynę? Przenieś całą swoją konfigurację za pomocą karty **Eksport / import ustawień**. Zobacz [Konfiguracja](configuration.md#portable-settings-export-and-import).
- Napotkałeś problem? Zobacz **[Rozwiązywanie problemów](troubleshooting.md)**.
