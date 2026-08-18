# Konfiguracja

Ta strona omawia zmienne środowiskowe kontenera, montaże udostępniane przez szablon, kopię VM przez SSH oraz konfigurację poza siedzibą. **Ścieżki repozytoriów** kopii są konfigurowane wewnątrz aplikacji (Ustawienia, Ścieżki kopii), a nie przez zmienne środowiskowe.

## Zmienne środowiskowe

| Zmienna | Wymagana | Opis |
|---|---|---|
| `APP_KEY` | **Tak** | 32-bajtowy sekret hex (64 znaki hex) używany do wyprowadzenia hasła repozytorium restic. Wygeneruj poleceniem `openssl rand -hex 32`. Chroń go: jego utrata sprawia, że zaszyfrowane kopie zapasowe stają się nieodzyskiwalne. |
| `LIBVIRT_HOST` | Dla VM | Host Unraid osiągany przez SSH do kopii VM (domyślnie `host.docker.internal`; szablon wstępnie wypełnia zastępczy adres IP w LAN). Użyj swojego IP Unraid w LAN, wymagane w niestandardowej sieci `br0.x`. |
| `LIBVIRT_SSH_PORT` | Nie | Port SSH hosta do kopii VM (domyślnie `22`). |
| `LIBVIRT_SSH_USER` | Nie | Użytkownik SSH na hoście do kopii VM (domyślnie `root`). |
| `LIBVIRT_URI` | Nie | Pełny URI połączenia libvirt, używany **dosłownie** zamiast budowania go z trzech powyższych zmiennych `LIBVIRT_*` (które są wtedy ignorowane przy tworzeniu ciągu połączenia). Domyślnie brak. Wymagany na TrueNAS Scale, którego libvirtd nasłuchuje na niestandardowym gnieździe, którego nie da się wyrazić w formie budowanego ciągu: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Zobacz sekcję TrueNAS Scale w [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Nie | Port HTTP (domyślnie `3000`; używany tylko z `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Nie | Port HTTPS (domyślnie `3443`; szablon publikuje go 1:1, więc WebUI odpowiada pod `https://<ip>:3443`). |
| `HTTP_ONLY` | Nie | Ustaw `true`, aby wyłączyć samopodpisany nasłuch HTTPS i serwować wyłącznie zwykły HTTP (do użytku za odwrotnym proxy terminującym TLS). |
| `HOST_SOURCE_ROOT` | Nie | Ścieżka hosta zamontowana jako **Host Data** (domyślnie `/mnt`). BombVault tłumaczy źródła montaży bind zgłaszane przez Docker na ścieżki pod tym montażem. Zmieniaj tylko, jeśli zamontowałeś inny katalog główny hosta. |
| `DATA_ROOT_SEGMENTS` | Nie | Lista nazw segmentów ścieżki oddzielonych przecinkami, które oznaczają źródło montażu bind jako dane kopii zapasowej (domyślnie `appdata`, zgodnie z konwencją Unraid `/mnt/user/appdata/<container>`). Montaż bind kontenera jest automatycznie wybierany do kopii, gdy KTÓRYKOLWIEK z wymienionych segmentów pojawia się jako pełny segment ścieżki jego źródła na hoście; na przykład `DATA_ROOT_SEGMENTS=appdata,config` obejmuje też montaż `.../config`. Zobacz [Wykrywanie źródeł kopii](#backup-source-detection), aby poznać inne, zawsze aktywne sposoby znajdowania folderu danych kontenera. |
| `PLATFORM` | Nie | Wymusza platformę, na której BombVault uznaje, że działa, zamiast wykrywać ją automatycznie: `unraid`, `generic` lub `truenas` (domyślnie brak: automatycznie wykrywa Unraid, sondując obecność jego znacznika `dockerMan` pod montowaniem flash, w przeciwnym razie `generic`; nierozpoznana wartość również powoduje użycie `generic`, co jest logowane). Ustaw ją jawnie na zwykłym hoście Docker lub TrueNAS Scale, zamiast polegać na automatycznym sondowaniu dostępnym tylko na Unraid. Plik compose dla zwykłego hosta robi to za Ciebie. Zmienia zastępczą konwencję appdata, domyślne miejsca docelowe przywracania między instancjami oraz to, czy w ogóle podejmowane są kroki powiadomień i pluginu towarzyszącego dostępne tylko na Unraid (zobacz `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Nie | Nazwa samego kontenera BombVault, aby nigdy nie tworzył kopii (a więc nie zatrzymywał) samego siebie (domyślnie `BombVault`; automatycznie wykrywana przez hostname w sieci bridge). |
| `BACKUP_MAX_HOURS` | Nie | Maksymalna liczba godzin rzeczywistego czasu, przez które pojedyncze uruchomienie kopii może trzymać blokadę swojej domeny, zanim zostanie wymuszenie anulowane (zabezpieczenie, aby zaklinowane uruchomienie nie mogło zablokować domeny na zawsze). Puste (domyślnie) używa `48`. Zwiększ dla bardzo dużych lub powolnych kopii w chmurze (uruchomienie anulowane na limicie kończy się błędem `context deadline exceeded`). Ustaw `0`, aby całkowicie wyłączyć limit. |
| `TZ` | Nie | Strefa czasowa dla harmonogramu (na przykład `Europe/Berlin`). |

## Montaże

Zamontuj gniazdo Docker, flash (`/boot`) oraz katalog główny **Host Data** (`/mnt`), jak pokazano w szablonie CA. Zarówno *źródła*, jak i *cele* kopii zapasowych znajdują się pod Host Data i jest on montowany jako **rslave**, więc zdalny udział, który montuje się po uruchomieniu kontenera (na przykład pod `/mnt/remotes`), staje się widoczny bez restartu.

Ścieżki repozytoriów kopii domyślnie wynoszą `/mnt/user/bombvault/{container,vms,flash,config,files}`, tworzone przy pierwszej kopii. Zmień lokalizację w dowolnym momencie w **Ustawienia, Ścieżki kopii**.

!!! note "Kontrola integracji z hostem"
    Otwórz `/spike` w interfejsie webowym po uruchomieniu kontenera. Sonduje ono każdy montaż i każde CLI (gniazdo Docker, libvirt, restic, qemu-img, rclone) i zgłasza wszelkie brakujące elementy.

## Model bezpieczeństwa

!!! warning "Kontrola nad hostem równoważna uprawnieniom root"
    Poprzez gniazdo Docker BombVault może zatrzymywać, usuwać i odtwarzać kontenery oraz odczytywać i zapisywać appdata, a w celu tworzenia kopii VM loguje się do hosta przez SSH (`qemu+ssh://`, domyślnie root), aby uruchomić `virsh`. Każdy, kto może dotrzeć do jego interfejsu webowego, ma faktycznie uprawnienia root na hoście.

- **Opcjonalna ochrona hasłem** (Ustawienia, Bezpieczeństwo): ustaw hasło, aby wymagać logowania, wyczyść je, aby wyłączyć. Domyślnie wyłączona do użytku w zaufanym LAN. Sesje są podpisywane (HMAC wyprowadzony z `APP_KEY`), a zmiana hasła je unieważnia; logowania mają ograniczoną częstotliwość.
- Ponieważ brama jest opcjonalna, gdy nie jest ustawiona, cały interfejs i API (w tym konfiguracja poza siedzibą, trasy tamper testu oraz zestaw odzyskiwania) są osiągalne dla każdego, kto może dotrzeć do portu. Włącz bramę, gdy tylko używasz kopii poza siedzibą, kopii niezmiennych lub szyfrowania.
- Uruchamiaj BombVault wyłącznie w zaufanej, nieudostępnianej na zewnątrz sieci. Do zdalnego dostępu umieść go za odwrotnym proxy dodającym uwierzytelnianie i TLS. Odpowiedzi niosą podstawowe nagłówki bezpieczeństwa (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Przy `HTTP_ONLY=true` ciasteczko sesji traci flagę `Secure` (musi, aby działać po zwykłym HTTP), więc włączaj hasło za proxy terminującym TLS tylko, jeśli poufność ma znaczenie.
- Połączenie SSH do kopii VM ufa kluczowi hosta przy pierwszym połączeniu (TOFU) i przypina go potem. Zweryfikuj klucz hosta poza pasmem, jeśli ścieżka od kontenera do hosta nie jest zaufana.
- Kopie zapasowe są szyfrowane przez restic, gdy szyfrowanie jest włączone (Ustawienia; domyślnie włączone), z kluczem wyprowadzonym z `APP_KEY`.

## Kopia VM przez SSH

BombVault tworzy kopie maszyn wirtualnych KVM/libvirt **bez montowania jakiejkolwiek ścieżki libvirt**. Uruchamia `virsh` na hoście przez SSH (`qemu+ssh://`), więc nigdy nie może wpłynąć na Twój host VM Manager.

Szybka konfiguracja:

1. **Ustawienia, System, Kopia VM przez SSH:** skopiuj pokazany klucz publiczny.
2. Dopisz go do pliku Unraid `/root/.ssh/authorized_keys` (utrwalanego też na flash, aby przetrwał restarty).
3. Kliknij **Test połączenia**.

Szablon dodaje `--add-host=host.docker.internal:host-gateway`, aby kontener mógł dotrzeć do hosta. Ustaw `LIBVIRT_HOST` na swoje IP Unraid w LAN, jeśli ta nazwa nie rozwiązuje się (na przykład gdy kontener działa w niestandardowej sieci `br0.x`). Jeśli zmieniłeś port SSH Unraid, ustaw `LIBVIRT_SSH_PORT`, aby pasował. **Migawki na żywo** dodatkowo wymagają agenta gościa qemu w VM oraz dysku na `/mnt/cache` (nie `/mnt/user`).

!!! important "Pełny przewodnik konfiguracji VM i sieci"
    Kompletny przewodnik krok po kroku (włączenie SSH, trwała autoryzacja klucza, routing sieci niestandardowej i VLAN, metoda per VM oraz rozwiązywanie problemów po stronie hosta) znajduje się pod [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) na GitHub.

## Konfiguracja poza siedzibą

Skonfiguruj replikę poza siedzibą w zakładce **Ustawienia, Poza siedzibą**. Zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md), aby poznać pełny przepływ pracy (niezmienne/append-only, tamper testy i próby DR). W skrócie:

- **Backendy:** SMB/CIFS i NFS (zamontuj udział i skieruj na niego Ścieżkę kopii), natywne backendy restic bez rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`) lub dowolny zdalny rclone (`rclone:<remote>:<bucket>/path`).
- **Poświadczenia chmurowe** są przechowywane zaszyfrowane w Ustawienia, Poza siedzibą, Poświadczenia chmurowe.
- **Cele SSH nie wymagają niczego zainstalowanego po drugiej stronie.** `sftp:` wymaga jedynie serwera SSH. Dodaj klucz publiczny z **Ustawienia, System, Kopia VM przez SSH** (dostępny też pod `/config/ssh/id_ed25519.pub`) do pliku `~/.ssh/authorized_keys` użytkownika docelowego.
- **Kopia poza siedzibą:** BombVault replikuje nowe migawki poleceniem `restic copy` w trybie best-effort. Repozytorium lokalne pozostaje główne. Każda domena ma własny harmonogram poza siedzibą oraz przycisk **Replikuj teraz**.
- **Wiele celów poza siedzibą na domenę:** każda domena może replikować do kilku celów poza siedzibą naraz. Dodaj dodatkowe cele w Ustawienia, Poza siedzibą, każdy z własnym repozytorium, klasą pamięci S3, flagą append-only, przechowywaniem i budżetem wzrostu; wszystkie replikują zgodnie z harmonogramem poza siedzibą tej domeny. Istniejąca pojedyncza konfiguracja poza siedzibą jest przenoszona jako pierwszy cel.
- **Przechowywanie per źródło:** polityka lokalna znajduje się w Ustawienia, Ścieżki i Magazyn; polityka poza siedzibą w Ustawienia, Poza siedzibą (pozostaw ją całą na zero, aby nigdy nie przycinać automatycznie migawek poza siedzibą).
- **Limity przepustowości:** ogranicz tempo wysyłania/pobierania restic w Ustawienia, Poza siedzibą.
- **Zimna i archiwalna klasa pamięci (S3):** dla natywnego repozytorium S3 poza siedzibą wybierz warstwę czytelną przy przywracaniu (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Zdalne rclone ustawiają swoją klasę w konfiguracji rclone.

## Przenośne ustawienia (eksport i import) {#portable-settings-export-and-import}

Karta **Eksport i import ustawień** na stronie Ustawienia zapisuje całą Twoją konfigurację BombVault (ustawienia domen, cele poza siedzibą, harmonogramy, przechowywanie, powiadomienia) do przenośnego pliku JSON, który możesz zaimportować na innej instancji, więc przeniesienie na nową maszynę lub sklonowanie konfiguracji nie oznacza ponownego wpisywania wszystkiego ręcznie. Import pokazuje podgląd i prosi o potwierdzenie oraz nigdy nie narusza Twoich danych ani historii kopii.

!!! warning "Eksport może zawierać poświadczenia"
    Sam decydujesz, czy dołączyć do pliku poświadczenia poza siedzibą i powiadomień. Z dołączonymi poświadczeniami eksport jest tak samo wrażliwy jak Twój zestaw odzyskiwania, więc przechowuj go w bezpiecznym miejscu. Bez nich plik zawiera tylko niesekretne ustawienia.
