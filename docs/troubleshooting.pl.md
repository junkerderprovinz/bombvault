# Rozwiązywanie problemów

Krótkie FAQ. Pełną tabelę rozwiązywania problemów po stronie hosta dla VM przez SSH (permission-denied, weryfikacja klucza hosta, brakujące zmienne szablonu i więcej) znajdziesz w [przewodniku Kopia VM przez SSH](vm-backup-ssh-setup.md).

## Coś nie jest poprawnie połączone {#something-is-not-wired-up-correctly}

Dodaj kartę **Integracja z hostem** do przeglądu w **Dostosuj**. Kontrola integracji z hostem sonduje każdy montaż i każde CLI (gniazdo Docker, libvirt, restic, qemu-img, rclone) i zgłasza wszelkie brakujące elementy, a **Sprawdź teraz** uruchamia ją ponownie. Zacznij tutaj, zanim założysz, że to błąd: brakujący montaż lub nieosiągalny host pojawia się natychmiast.

## Nie mogę dotrzeć do interfejsu webowego {#i-cannot-reach-the-web-ui}

BombVault serwuje HTTPS od razu po instalacji na porcie `3443` (certyfikat samopodpisany), więc otwórz `https://<your-unraid-ip>:3443`. Zaakceptuj ostrzeżenie o certyfikacie samopodpisanym lub umieść BombVault za odwrotnym proxy z własnym certyfikatem. Jeśli uruchamiasz z `HTTP_ONLY=true`, serwuje zamiast tego zwykły HTTP na porcie `3000` (przeznaczony do użytku za proxy terminującym TLS).

## Zgubił się mój APP_KEY {#i-lost-my-app_key}

`APP_KEY` wyprowadza hasło repozytorium restic. Bez niego, bez zestawu odzyskiwania i bez depozytu klucza w Twojej grupie zaszyfrowanych kopii nie da się odzyskać. Dlatego przegląd pokazuje **Zapisz swój zestaw odzyskiwania**, dopóki go nie pobierzesz. Zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md#recovery-kit). Wygeneruj klucz poleceniem `openssl rand -hex 32` i przechowuj go poza serwerem, zanim zaczniesz polegać na jakiejkolwiek kopii.

## Zapomniane hasło logowania {#i-forgot-the-login-password}

Wybierz **Nie pamiętasz hasła?** na stronie logowania. BombVault zapisuje kod z 12 znaków w logu swojego kontenera (w Unraid: zakładka Docker, ikona BombVault, **Logs**). Wpisz go razem z nowym hasłem w ciągu 10 minut. Reset wyłącza też uwierzytelnianie dwuskładnikowe, usuwa każdy klucz dostępu (passkey), unieważnia każdy token API, klucz MCP i logowanie OAuth, kończy wszystkie inne sesje i wyłącza parowanie telefonów. Dodaj swoje klucze dostępu ponownie w **Ustawienia, Bezpieczeństwo**, włącz tam z powrotem parowanie telefonów i utwórz nowe tokeny i klucze tam, gdzie jeszcze ich potrzebujesz. Zobacz [Model bezpieczeństwa](configuration.md#login).

## Kopie zapasowe się zatrzymały i mówią, że klucza nie można już otworzyć {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Ten serwer przejął w Kreatorze awaryjnym klucz kopii innego serwera, a potem zmieniono `APP_KEY`. Przejęty klucz jest zapieczętowany starym APP_KEY, więc BombVault nie może go otworzyć i raczej niczego nie kopiuje, niż miałby zapisywać złym kluczem. **Wymaga uwagi** w przeglądzie go wymienia. Ustaw APP_KEY z powrotem na dawną wartość albo wgraj zestaw odzyskiwania ponownie na stronie Przywracanie w **Awaria**.

## Telefon trafia na stronę logowania {#a-phone-lands-on-the-login-page}

Sparowany telefon dostaje sesję przez grupę tylko wtedy, gdy w **Ustawienia, Bezpieczeństwo** włączone jest **Telefony mogą się sparować z dwunastoma słowami grupy**. Reset hasła je wyłącza. Aplikacja starsza niż serwer też nie dostaje sesji, dopóki nie zostanie zaktualizowana. Serwer dodany po adresie zawsze pyta o hasło. Telefon usunięty ze strony Instancje wraca wyłączony, gdy jego aplikacja znowu się otworzy. Aby go wpuścić, włącz **Odpytuj tę instancję** w sekcji **Monitorowanie** w jego szczegółach.

## Kopia VM nie łączy się {#vm-backup-will-not-connect}

Kopia VM komunikuje się z libvirt przez SSH, nigdy przez montaż.

- Potwierdź, że SSH jest włączone na hoście, a klucz publiczny BombVault jest autoryzowany w `/root/.ssh/authorized_keys` (Ustawienia, Integracje, SSH hosta pokazuje klucz oraz przycisk **Testuj połączenie**).
- W niestandardowej sieci `br0.x` ustaw `LIBVIRT_HOST` na swoje IP Unraid w LAN (kontener nie może tam dotrzeć do hosta przez `host.docker.internal`). Włącz **Settings, Docker, Host access to custom networks**.
- Jeśli zmieniłeś port SSH Unraid, ustaw `LIBVIRT_SSH_PORT`, aby pasował.
- Na TrueNAS ustaw `LIBVIRT_URI` z własnym gniazdem libvirt TrueNAS, nadaj aplikacji uprawnienie `SETPCAP` i dodaj klucz do roota w **Credentials, Users** (zobacz [TrueNAS SCALE](getting-started.md#truenas)).
- Pełna diagnoza krok po kroku (test osiągalności, routing VLAN, `Permission denied (publickey)`, `Host key verification failed`) znajduje się w [przewodniku Kopia VM przez SSH](vm-backup-ssh-setup.md).

## Migawka VM na żywo nie została uruchomiona {#a-live-vm-snapshot-did-not-run}

Migawki na żywo wymagają agenta gościa qemu zainstalowanego w VM oraz dysku na `/mnt/cache` (lub `/mnt/diskX`), nie `/mnt/user`. Na wyłączonej VM tryb na żywo automatycznie wraca do płynnego. Płynna kopia wyłącza VM, tworzy kopię dysków, a następnie ją restartuje, więc jest zawsze spójna. Czeka do dziesięciu minut, aż gość się wyłączy, i nigdy nie wyłącza go siłą, więc gość Windows instalujący aktualizacje nie zostaje przerwany w połowie. Gość, który po tym czasie wciąż działa, nie jest ruszany, a ta kopia kończy się błędem z podaniem przyczyny.

## Kopia zapasowa zawiodła z "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

To zwykle osierocona blokada restic pozostawiona, gdy kontener został zaktualizowany lub zrestartowany w trakcie operacji. BombVault wykrywa bezspornie osieroconą blokadę, wymuszenie ją usuwa i ponawia raz, automatycznie. Jeśli się utrzymuje, otwórz kartę **Sprawdź kopie zapasowe** w **Ustawienia, Weryfikacja** i wybierz **Odblokuj** pod **Więcej** w wierszu dotkniętej sekcji, aby ręcznie usunąć nieaktualną blokadę. Prawdziwy problem nadal wychodzi na jaw zamiast być ukrywany. Po restarcie BombVault czeka, aż taka blokada przez dziesięć minut nie zostanie odświeżona. Restic, który wciąż działa, na przykład w drugim BombVaulcie na tym samym repozytorium, odświeża swoją blokadę co pięć minut.

## Moja kopia poza siedzibą nie zdarzyła się po kopii {#my-off-site-copy-did-not-happen-after-a-backup}

Replikacja poza siedzibą jest z założenia best-effort, więc potknięcie poza siedzibą nigdy nie powoduje niepowodzenia kopii lokalnej. Sprawdź, kiedy miejsce docelowe kopiuje: jego karta **Kopiuj** mówi **Po każdej kopii zapasowej** albo wymienia własny plan, a **Ustawienia, Harmonogram, Kopie poza siedzibą według sekcji** może dać sekcji własny plan. Użyj **Replikuj teraz** w wierszu sekcji w karcie **Kopie poza siedzibą** w **Ustawienia, Miejsca docelowe** do uruchomienia na żądanie i obserwuj nagłówek stanu przeglądu, który w trakcie pisze **Trwa kopiowanie poza siedzibę**. Nieudana kopia pojawia się pod **Wymaga uwagi**.

## Certyfikat rest-servera nie jest zaufany {#a-rest-servers-certificate-is-not-trusted}

restic odrzuca certyfikat, którego nie zna, a komunikat błędu mówi, gdzie go umieścić. Skopiuj samopodpisany certyfikat rest-servera, albo certyfikat urzędu, który go podpisał, jako plik PEM do folderu `certs` w folderze danych BombVault (`/config/certs` w kontenerze, `appdata/bombvault/certs` w Unraid) i przetestuj ponownie. Restart nie jest potrzebny.

## Kopia zapasowa zgłasza, że dane poza Host Data nie zostały zarchiwizowane {#data-outside-host-data}

Kontener trzyma dane w folderze, którego BombVault nie może odczytać przez Host Data. Kopia zapisuje definicję i każdy folder, do którego sięga, barwi swój wiersz w dzienniku aktywności na pomarańczowo i wymienia foldery, które pominęła. Dwa przypadki są częste: ixVolume na TrueNAS pod `/mnt/.ix-apps` oraz nazwany wolumen Dockera pod `/var/lib/docker/volumes`, na przykład wolumen bazy danych stosu compose. Karta **Foldery do kopii** na karcie **Ustawienia** kontenera pokazuje każdy taki folder z jego ścieżką wewnątrz kontenera i ścieżką na hoście, oznaczony na czerwono.

Anonimowy wolumen Dockera nie jest zgłaszany. Docker sam go tworzy dla folderu zadeklarowanego przez obraz, którego nikt nie zmapował, nadaje mu nazwę z 64 cyfr szesnastkowych i trzyma go pod `/var/lib/docker/volumes`. Zwykle jest w nim pamięć podręczna, więc kopia pozostaje zielona, a karta tylko go pokazuje z uwagą, że jest anonimowy.

- **Chcesz archiwizować te dane.** Zmapuj folder na ścieżkę hosta pod Host Data, na przykład na folder w udziale appdata, a następna kopia go obejmie.
- **Nie potrzebujesz ich.** Dodaj jedną z jego dwóch ścieżek jako wiersz w karcie **Wzorce wykluczeń** na tej samej karcie: ścieżkę wewnątrz kontenera, na przykład `/cache`, albo ścieżkę na hoście, którą podaje ostrzeżenie, na przykład `/var/lib/docker/volumes/jellyfin_cache/_data`. Od następnej kopii ostrzeżenie pomija ten folder, a podgląd pod polem mówi dlaczego. Wiersz dla folderu nadrzędnego też działa, sama nazwa ani `/` nie.

## Kopia zapasowa ma stan Odroczono {#deferred-backups}

Aplikacja kontenera była zajęta, więc jego kopia czeka i jest ponawiana samoczynnie. Wiersz w dzienniku aktywności mówi, dlaczego i kiedy będzie następna próba. Karta **Gdy aplikacja jest zajęta** na karcie **Ustawienia** kontenera mówi to samo oraz kiedy kończy się oczekiwanie. Nie trzeba nic robić. Aby zrobić kopię od razu, użyj **Utwórz kopię teraz**: kopia uruchomiona przez ciebie nigdy nie czeka, także przy kodzie wyjścia 75.

- **Własna kontrola.** Spraw, by **Polecenie przed kopią** kończyło się kodem 75, gdy aplikacja jest zajęta, na przykład `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Każdy inny kod różny od 0 nadal powoduje niepowodzenie kopii, więc zepsuty skrypt wciąż pokazuje się w **Wymaga uwagi**.
- **Plex, Jellyfin albo Emby nigdy nie dostaje pytania.** BombVault najpierw pyta serwer pod własnym adresem kontenera, a gdy tam nie ma odpowiedzi, uruchamia curl albo wget wewnątrz kontenera. Obrazu bez żadnego z nich nie da się zapytać przez niego samego. Dziennik wtedy to podaje. Przy włączonym **Twórz kopię, gdy serwer nie odpowiada** taka kopia i tak się wykonuje, przy wyłączonym czeka.
- **Jellyfin albo Emby odmawia.** Sprawdź **Klucz API**. Tworzy się go w panelu serwera w sekcji kluczy API.
- **Plex odmawia.** BombVault odczytuje token z `Preferences.xml` w folderze zamontowanym jako `/config`. Serwer, na którym nikt się nigdy nie zalogował, nie ma tokenu i odpowiada tylko tam, gdzie pozwala na dostęp bez niego.
- **Wstrzymany film każe kopii czekać całą noc.** Ustaw **Ignoruj wstrzymane sesje po (min)**, na przykład na 30.
- **Kontrola ruchu nigdy nie przepuszcza kopii.** Podnieś **Zajęty od (KiB/s)** powyżej tego, co kontener wysyła, gdy nikt go nie używa. Zaraz po starcie BombVault kontrola czeka, dopóki nie poobserwuje kontenera przez **Spokój przez (min)** minut.
- **Kopia nigdy nie wykonuje się w nocy.** Gdy okno jest ustawione, próby odbywają się tylko w nim. Oczekiwanie, które osiąga termin poza oknem, i tak kończy się w terminie.

## Duża kopia zapasowa została zabita z braku pamięci {#a-large-backup-was-killed-for-running-out-of-memory}

Ustaw **Limit pamięci na kopię zapasową (MiB, 0 = brak)** w karcie **Wydajność** w **Ustawienia, System** wyraźnie poniżej tego, ile pamięci może używać kontener. W pobliżu limitu restic częściej zwalnia pamięć, co kosztuje trochę szybkości, więc kopia działa wolniej, zamiast zostać zabita.

## Przywracanie przerwane, zanim się zaczęło {#a-restore-aborted-before-it-started}

Zanim cokolwiek zostanie zatrzymane lub usunięte, przywracanie uruchamia kontrolę konfliktów przed startem: weryfikuje, czy statyczny IP kontenera oraz opublikowane porty hosta są wolne. Jeśli inny kontener już jeden z nich zajmuje, przerywa działanie z czytelnym, praktycznym komunikatem zamiast pozostawiać niedokończone przywracanie. Zwolnij konfliktujący port lub IP, a następnie ponów.

Jeśli sieć kontenera nie istnieje, na przykład dlatego, że usunięto jego aplikację TrueNAS, kontrola przed przywracaniem podaje tę sieć, a przywracanie odmawia, zanim cokolwiek zmieni. Zainstaluj aplikację ponownie w TrueNAS pod tą samą nazwą, co tworzy sieć na nowo, a potem przywróć jej dane.

## Jawny eksport zawiódł zamiast zapisać plik {#a-plain-export-failed-instead-of-writing-a-file}

Jeśli **Szyfruj eksporty za pomocą age** jest włączone w **Ustawienia, System**, ale nie ustawiono prawidłowego odbiorcy, eksport kończy się czytelnym błędem zamiast zapisać tekst jawny. Dodaj prawidłowego odbiorcę (klucz publiczny age lub klucz publiczny SSH) albo wyłącz szyfrowanie, jeśli chcesz, aby eksport był w postaci jawnej. Zobacz [Zwykłe eksporty](backups.md#exports).

## Zrzut bazy danych się nie udał {#a-database-dump-failed}

Nieudany zrzut nigdy nie przewraca kopii, w której powstał. Zapisuje się jako własny nieudany przebieg, a powód mówi, co naprawić.

- **Odmowa logowania.** Zrzut loguje się zmiennymi hasła samego kontenera (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` albo ich odpowiednikami `_FILE`). Sprawdź je na kontenerze bazy. Zmienna `_FILE` wskazująca sekret, którego użytkownik kontenera nie może odczytać, kończy się tak samo.
- **Brak uprawnień.** Przy losowym haśle roota zrzut może zalogować się tylko jako użytkownik aplikacji, obejmuje więc tę jedną bazę, a MySQL 8.4 i nowsze mogą odmówić całkiem. Nadaj kontenerowi prawdziwe hasło roota albo wyłącz mu zrzut.
- **Tabele systemowe wymagają aktualizacji.** MariaDB odmawia zrzutu, gdy jej tabele systemowe pochodzą ze starszej wersji (błąd 1558). Dodaj zmienną `MARIADB_AUTO_UPGRADE=1` i zrestartuj kontener albo uruchom w nim raz `mariadb-upgrade`.
- **Brak narzędzia do zrzutu.** Odchudzonego lub własnoręcznie zbudowanego obrazu bez `pg_dump`, `mysqldump` czy `mariadb-dump` nie da się zrzucić. Weź obraz oficjalny albo wyłącz zrzut.
- **Limit czasu.** Zrzut dostaje `DB_DUMP_MAX_HOURS` (domyślnie 6), kopia wokół niego `BACKUP_MAX_HOURS`, a zrzut, który przestaje robić postępy, zostaje ucięty po `BACKUP_STALL_HOURS`. Za tym ostatnim stoi zwykle blokada trzymana przez aplikację. Podnieś limit, który zadziałał, albo zrzucaj, gdy aplikacja jest spokojna.
- **Kontener jest wstrzymany albo się restartuje.** Zrzut rozmawia z działającym serwerem. Jeśli kontener restartuje się w kółko, jego własny dziennik mówi dlaczego.
- **Uszkodzonego zrzutu nie dało się usunąć.** Zrzut, którego BombVault nie zdołał dokończyć, jest kasowany. Gdy to kasowanie się nie uda, zrzut zostaje na liście oznaczony jako uszkodzony i możesz go tam usunąć.

## Import się nie udał {#an-import-failed}

Import zatrzymuje kontener, odsuwa jego folder danych na bok i pozwala obrazowi utworzyć w tym miejscu pusty. Jeśli zawiedzie krok przed samym importem, stary folder wraca na miejsce samoczynnie. Jeśli zawiedzie import, kontener zostaje z nowym folderem, a stary leży obok jako `<folder danych>.bombvault-before-import-<znacznik czasu>`. Komunikat błędu przebiegu podaje dokładną ścieżkę.

Folderowi danych, który jest osobnym zbiorem danych, jak na TrueNAS, nie da się zmienić nazwy. Import zostawia wtedy folder na miejscu i przenosi jego zawartość do zachowanego folderu obok, więc nadrzędny zbiór danych potrzebuje na nią miejsca. Import sprawdza to, zanim zatrzyma kontener.

Ręczne przywrócenie: zatrzymaj kontener, zmień nazwę bieżącego folderu danych, żeby zszedł z drogi, przywróć zachowanemu folderowi pierwotną nazwę i uruchom kontener. W przypadku zbioru danych przenieś zawartość zamiast zmieniać nazwy folderów. Na Unraidzie robi to menedżer plików w zakładce Shares.

## Kopia zbioru danych ZFS się nie udała albo pominęła zbiór {#zfs-datasets}

Każdy problem ma kod przyczyny w nawiasach kwadratowych, a strona [Zbiory danych ZFS](zfs-datasets.md#reason-codes) wymienia wszystkie wraz z rozwiązaniem. Trzy najczęstsze:

- **`snapshot-loop`**: migawka nie dotarła do BombVault, bo Host Data nie przekazuje nowych montowań. Edytuj kontener, ustaw Access Mode dla Host Data na Read/Write - Slave i uruchom ponownie BombVault.
- **`key-not-loaded`**: zaszyfrowany zbiór, którego klucz nie jest załadowany, jest pomijany. Załaduj klucz poleceniem `zfs load-key` i zamontuj zbiór. Następna kopia go obejmie.
- **`ssh-auth`**: serwer odrzucił klucz BombVault. **Połączenie ZFS z tym serwerem**, otwierane przez **Pokaż połączenie** na stronie Kopie zapasowe, pokazuje polecenie, które go autoryzuje. Uruchom je raz na serwerze.

## Element zatrzymuje się na "Uczy się N/10" {#an-entry-stays-at-learning-n10}

Większość kontroli anomalii zaczyna się po 10 udanych kopiach elementu, a licznik zaczyna od nowa po **Oznacz jako oczekiwane** i po zmianie wyboru elementu. Element bez harmonogramu się nie uczy, a kontener bez appdata nie ma się na czym uczyć, co mówi też jego znaczek. **Postęp nauki** na stronie Anomalie pokazuje pierścień z dziesięciu części dla każdego elementu.

## Retencja przestała usuwać stare kopie jednego elementu {#retention-stopped-deleting-old-backups-of-one-entry}

Wstrzymuje je otwarta krytyczna anomalia: źródło elementu jest prawie puste, mocno się skurczyło albo kopia zapisała ponownie większość danych. Otwórz anomalię ze znaczka przy elemencie. Jeśli brakuje danych albo zostały zaszyfrowane, najpierw przywróć je z podlinkowanej ostatniej dobrej kopii. Potem potwierdź anomalię albo oznacz ją jako oczekiwaną, jeśli zmiana była twoja, a następny przebieg posprząta jak zwykle. Podgląd retencji oznacza taki element jako zachowany. W elemencie ZFS stare kopie zachowuje tylko zbiór danych wskazany w anomalii; pozostałe zbiory drzewa są czyszczone jak zwykle.

## Ręczne czyszczenie mówi, że niektóre elementy zostały zachowane {#manual-prune-says-some-entries-were-kept}

Ta sama przyczyna: czyszczenie nie rusza starych kopii elementu z taką anomalią i wymienia go w swoim komunikacie. Wszystko inne jest czyszczone jak zwykle.

## Import historii mówi, że nie udało się odczytać repozytorium {#history-import-says-a-repository-could-not-be-read}

Po aktualizacji BombVault raz odczytuje rozmiary wcześniejszych kopii z każdego repozytorium. Repozytorium, które było wtedy nieosiągalne, na przykład niedziałające zdalne miejsce docelowe albo niezamontowany udział, pojawia się na karcie **Anomalie** w **Ustawienia, Weryfikacja** i jest ponawiane raz dziennie. W tym czasie jego elementy uczą się na nowych kopiach.

## Ostrzeżenie o miejscu na dysku nie zgadza się z pulpitem Unraid {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Na udziale użytkownika Unraid (`/mnt/user`) wolne miejsce to wolne miejsce całej macierzy, a nie jednego dysku. Zdalne miejsca docelowe, do których BombVault dociera przez rclone, jak SFTP, Hetzner Storage Box, SMB, WebDAV czy dysk w chmurze, są mierzone, gdy druga strona podaje swoje wolne miejsce, tak samo jak wpisany ręcznie adres `sftp:`. Repozytoria S3, B2 i REST nie mają wartości: ich miejsce docelowe mówi **Rozmiar nieznany**, a karta **Anomalie** wymienia je jako niezmierzone.

## Asystent AI nie może się połączyć {#an-ai-assistant-cannot-connect}

Strona [Serwer MCP](mcp.md#troubleshooting) wyjaśnia, co oznacza każdy kod stanu i każda odmowa punktu końcowego MCP oraz co z tym zrobić.

## Kontener wciąż się restartuje lub wygląda na niesprawny {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault zgłasza stan healthy/unhealthy z własnego `/api/health`. Narzędzie do auto-naprawy (takie jak Autoheal) może go automatycznie zrestartować, jeśli silnik kiedykolwiek się zaklinuje. Sprawdź log kontenera oraz kartę **Integracja z hostem** w poszukiwaniu przyczyny źródłowej.

## Miejsce docelowe nie chce się zapisać {#location-refused}

BombVault sprawdza miejsce, zanim je zapisze, a odmowa mówi jednym zdaniem dlaczego. Zwykłe powody:

- **„This location lies inside another repository or target, or contains one.”** Repozytorium wewnątrz repozytorium zepsułoby oba. Komunikat podaje to drugie miejsce. Wybierz inny folder albo najpierw usuń tamto.
- **To repozytorium, które sam ustawiłeś.** Miejsce jest jednym z wierszy pod **Repozytoria** albo leży wewnątrz jednego z nich lub wokół niego. Wybierz inne miejsce albo najpierw usuń to repozytorium.
- **To samo miejsce co własny folder sekcji.** Kopia na własnym folderze sekcji kopiowałaby jej kopie zapasowe na nie same przy każdym przebiegu, więc jest odrzucana. Wybierz inne miejsce albo zobacz [Własny folder sekcji](offsite-recovery.md#own-folder).
- **„That is not an off-site target of this domain”** pochodziło z linku do miejsca, które usunięto. Aktualna wersja czyści takie linki na starcie i przy imporcie. Jeśli nadal to widzisz, zaktualizuj BombVault.
- **Pełna ścieżka hosta**, na przykład `/mnt/user/backups`. BombVault działa w kontenerze, więc chce ścieżki pod Host Data, `user/backups`, a komunikat podaje tę, którą wpisać.

## Zdalne miejsce docelowe mówi, że jego repozytorium zniknęło {#repository-gone}

Miejsce, które miało kopie i nagle nie ma repozytorium, nie dostaje samo z siebie nowego, pustego, bo tamto ukryłoby prawdziwe, gdy dysk wróci. Zwykle winny jest rest-server, którego dysk się nie zamontował, więc zamontuj dysk ponownie i uruchom kopię albo kopiowanie jeszcze raz. Tylko jeśli stare repozytorium opróżniłeś lub usunąłeś celowo, naciśnij **Utwórz tu nowe repozytorium** na stronie miejsca. Zaczyna od nowa dla wszystkich sekcji tego miejsca naraz.

## Logowanie do chmury wygasło {#cloud-sign-in-expired}

Test albo następne kopiowanie mówi wtedy, że logowanie wygasło. **Zmień połączenie** na stronie miejsca loguje od nowa.

- **Box, Jottacloud i inne z jednorazowymi tokenami odświeżania:** BombVault zachowuje token, który rclone odświeżył, także po restartach i w zestawie odzyskiwania. Po przywróceniu ze starszej kopii ustawień token w niej mógł wygasnąć, więc zaloguj się jeszcze raz. Repozytorium zostaje, jakie jest.
- **iCloud Drive:** Apple kończy logowanie po 30 dniach. Potrzebny jest nowy token zaufania i ciasteczka z `rclone config`.
- **OneDrive:** logowanie z kilkoma dyskami pyta, którego użyć, więc miejsce bez wyboru pozostaje bezużyteczne, dopóki nie wybierzesz. Dla OneDrive Business i Google Drive zobacz uwagę o przyciętych danych w [Miejsca docelowe](offsite-recovery.md#destinations).

## „Kopiowanie poza siedzibę ostatniej kopii jeszcze trwa” {#copy-still-running}

Poprzednia kopia się skończyła, a jej kopiowanie jest jeszcze w drodze, co blokuje nową kopię tej sekcji. Poczekaj, aż nagłówek stanu przestanie mówić **Trwa kopiowanie poza siedzibę**. Przyciski, które uruchamiają kopię, mówią to samo, gdy czekają. Pierwsze kopiowanie do nowego miejsca może trwać godzinami, zobacz [przykład z S3](offsite-recovery.md#example-s3).

## Replika ZFS zatrzymuje się, bo cel ma nowsze migawki {#replica-target-newer}

Cel ma migawki nowsze niż najnowsza, którą dzieli z tym serwerem, na przykład dlatego, że zbiór danych tutaj cofnięto albo zmieniono kopię na celu. Karta Replika i historia przebiegów podają migawkę pod **Cofnij miejsce docelowe do**. Uruchom `zfs rollback -r` do tej migawki na celu, dla każdego zbioru danych, który wymienia przebieg, i naciśnij **Replikuj teraz**. Zobacz [Jak długo zostają migawki](zfs-datasets.md#replica-retention).

## Maszyna wirtualna z migawkami Unraid zawodzi i podaje plik {#vm-chain}

BombVault podąża za łańcuchem bazowym każdego dysku i kopiuje każdy plik w nim. Łańcuch, za którym nie da się dojść do końca, kończy kopię błędem zamiast pominąć część dysku, a komunikat podaje plik. Sprawdź, czy plik istnieje pod `/mnt` i czy migawka jest nadal wymieniona w VM Manager. Pliku spoza Host Data nie da się odczytać w ogóle. Zobacz [Maszyny wirtualne z migawkami Unraid](vm-backup-ssh-setup.md#unraid-snapshots).

## Kopie poza siedzibą są wstrzymane po odbudowie {#copies-paused}

Po **Odkryj kopie zapasowe** albo Kreatorze awaryjnym bez przywróconych ustawień kopie poza siedzibą każdej odbudowanej sekcji czekają, bo BombVault nie wie już, które elementy pominąłeś. Mówi o tym przegląd. Naciśnij **Potwierdź domyślne** w **Miejsca docelowe według sekcji**, sprawdź podgląd i pomiń to, co ma zostać pominięte. Import pliku ustawień nie kończy wstrzymania. Zobacz [Po odbudowie](offsite-recovery.md#after-a-rebuild).

## Powiadomienia nie docierają {#notifications-do-not-arrive}

Zacznij od **Wyślij test** w **Ustawienia, Powiadomienia**. Linia pod kartą mówi, co się stało.

- **Nic nie jest wysyłane.** Sprawdź **Powiadom**. Przy **Nigdy** nic nie wychodzi, a przy **Tylko przy niepowodzeniu** udany przebieg milczy. Przy włączonym **Podsumowuj zaplanowane przebiegi** zaplanowany przebieg wysyła na końcu jedno podsumowanie.
- **Nowy adres.** Po zmianie serwera Matrix, Gotify albo poczty pole prosi o token lub hasło ponownie, a nowy adres zapisuje się dopiero po ich wpisaniu. BombVault nie wysyła starego sekretu w nowe miejsce.
- **Gotify** mówi, że serwer odrzucił token aplikacji: skopiuj go ponownie z aplikacji w Gotify. **URL serwera** to adres bazowy, bez `/message`.
- **Webhook:** sprawdź, czy **Format Webhook** pasuje do usługi. Adres Discorda wymaga **Discord**, a ntfy wymaga **ntfy**.
- **Powiadomienia Unraid** działają przez połączenie SSH z hostem, więc przetestuj je najpierw w **Ustawienia, Integracje**. Na innej platformie pozostają wyłączone, a karta o tym mówi.
- **Wiadomość e-mail nigdy nie przychodzi.** Sprawdź **Szyfrowanie** względem portu, zwykle **STARTTLS** na 587 albo **TLS (implicit)** na 465.

Pingi Healthchecks od niczego z tego nie zależą, zobacz [Healthchecks](notifications.md#healthchecks).

## Aplikacja na Androida nie może otworzyć serwera {#android-cannot-open}

Ekran startowy mówi na karcie serwera dlaczego:

- **Brak połączenia:** nie odpowiada ani adres serwera, ani relay. Przy **Bez relay** aplikacja dociera do serwera tylko pod jego własnym adresem.
- **Sprawdź certyfikat:** serwer pokazuje inny certyfikat niż ten, któremu zaufałeś. Aplikacja otwiera go dopiero po zaufaniu nowemu, więc upewnij się, że zmiana jest Twoja.
- **Wymagane logowanie:** serwer dodany po adresie pyta o hasło. Zobacz też [Telefon trafia na stronę logowania](#a-phone-lands-on-the-login-page).
- **Klucz się zmienił:** ekran startowy pokazuje odcisk nowego klucza, a strona otwiera się dopiero po zaufaniu mu. Zobacz [Poza domem](android.md#away).

## Inny użytkownik nie może odczytać lokalnego repozytorium {#repository-permissions}

Lokalne repozytorium pozostaje czytelne dla innych użytkowników, takich jak użytkownik synchronizacji przez SMB. Przed 10.4.1 pliki zapisane przez przycinanie, tag, pobieranie, przeniesienie sekcji, usunięcie albo kopię stosu mogły pozostać nieczytelne do następnej nocy. Zaktualizuj BombVault, a będą czytelne od razu. Repozytorium autokopii pozostaje celowo czytelne tylko dla roota, bo zawiera ustawienia BombVault.

## Nadal utknąłeś? {#still-stuck}

- Przeczytaj pełne strony [Konfiguracja](configuration.md) i [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md).
- **Zgłoszenie błędu** w karcie **Pomoc** w **Ustawienia, Info** pobiera ZIP do zgłoszenia na GitHubie: sprawdzenie hosta, Twoją konfigurację z usuniętymi wszystkimi sekretami, ostatnie przebiegi, to, co jest zaplanowane dalej, i ostatni log od ostatniego startu kontenera. Zawiera też ostatni zrzut każdej bazy danych, elementy ZFS z ich montowaniami, otwarte anomalie i liczbę istniejących kluczy MCP. Hasła, tokeny i konfiguracja rclone są usunięte, a paczka mówi o tym we własnym manifeście. Wymaga hasła logowania. Przy awarii, która zrestartowała kontener, zajrzyj do `docker logs`.
- Zapytaj na [wątku wsparcia Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Otwórz [zgłoszenie na GitHub](https://github.com/junkerderprovinz/bombvault/issues).
