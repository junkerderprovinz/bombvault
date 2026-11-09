# Kopie zapasowe i elementy

Wszystko, co BombVault kopiuje, stoi na jednej liście pod **Kopie zapasowe**: kontenery, maszyny wirtualne, zestawy folderów, dysk flash, zbiory danych ZFS i autokopia. Każde z nich to **element** z własną stroną. Ta strona prowadzi przez listę, stronę elementu z jej trzema kartami i ustawienia, jakie może mieć element.

![Lista Kopie zapasowe z wybranym kafelkiem Containers: każdy wiersz pokazuje ostatnie przebiegi jako pasek i jeden stan.](assets/screenshots/backups/list-dark.png#only-dark)
![Lista Kopie zapasowe z wybranym kafelkiem Containers: każdy wiersz pokazuje ostatnie przebiegi jako pasek i jeden stan.](assets/screenshots/backups/list-light.png#only-light)

## Co zapisuje każda sekcja {#what-is-saved}

| Sekcja | Co jest zapisywane |
|---|---|
| **Containers** | Folder appdata oraz definicja kontenera: obraz, zmienne, porty, etykiety i wolumeny. Karta **Foldery do kopii** na karcie **Ustawienia** kontenera zaznacza dokładnie, które foldery obejmuje kopia, z bieżącą liczbą ścieżek i listą tego, co pominąłeś. Dane, które kontener trzyma poza Host Data, na przykład w ixVolume na TrueNAS albo w projekcie Compose pod `/opt`, nie są kopiowane, a przebieg wymienia pominięte foldery. Przebieg wymienia też folder wewnątrz Host Data, którego w tej chwili nie ma. Ścieżki systemowe, takie jak `/etc/localtime` czy gniazdo Docker, nie liczą się jako dane. |
| **Maszyny wirtualne** | Obrazy dysków, definicja XML i UEFI NVRAM, z łagodnym zamknięciem albo migawką na żywo, przez SSH. Zobacz [Kopie maszyn wirtualnych](vm-backup-ssh-setup.md). |
| **Flash** | Tylko Unraid. Cały dysk USB flash (`/boot`): system, licencja, konfiguracja macierzy, udziały, sieć i konfiguracja wtyczek. |
| **Autokopia** | Własny `/config` BombVault: jego baza ustawień, logowania do miejsc docelowych, para kluczy SSH libvirt, klucz i przypięte klucze hosta serwerów ZFS oraz certyfikaty, zapisane migawką SQLite `VACUUM INTO`, żeby baza nigdy nie została skopiowana w trakcie zapisu. |
| **Foldery** | Dowolny folder na serwerze jako zestaw folderów, na przykład udział, Twoje dokumenty albo biblioteka zdjęć, z opcjonalnymi wzorcami wykluczeń. Zestawy folderów mają harmonogramy, przechowywanie, kopie poza siedzibą, kontrole i próby przywracania jak wszystko inne. |
| **Zbiory danych ZFS** | Zbiór danych i każdy zbiór poniżej niego, odczytane z jednej migawki ZFS, żeby wszystkie pochodziły z tej samej chwili, i zapisane przez restic jak folder. Element ZFS może też trzymać replikę co do bloku na drugim serwerze ZFS. Zobacz [Zbiory danych ZFS](zfs-datasets.md). |

Każda kopia przechodzi przez restic, więc jest przyrostowa i zdeduplikowana, a nawet duże dyski maszyn wirtualnych nie rozdymają repozytorium. BombVault nigdy nie kopiuje, a więc nigdy nie zatrzymuje, własnego kontenera. Przebieg można bezpiecznie zatrzymać, bo restic zapisuje migawkę na końcu, więc przerwany przebieg zostawia dane bez odwołań, ale żadnej migawki. Blokada pozostawiona przez restart w środku przebiegu jest usuwana, a przebieg próbowany jeszcze raz, i to dopiero wtedy, gdy blokada nie była odświeżana przez dziesięć minut.

## Lista {#list}

Kafelki nad listą wybierają rodzaj, na przykład **Containers** albo **ZFS**, a każdy kafelek liczy swoje elementy. Kafelek wybiera, nie ukrywa reszty strony.

Każdy wiersz pokazuje czternaście ostatnich przebiegów elementu jako małe kwadraty i jeden stan, na przykład **Chronione**, **Nie chronione**, **Odroczono** albo **Replikowanie**. Wszystko inne jest w dymku informacyjnym obok nazwy, a gdy wskaźnik spoczywa na wierszu, ten tekst pojawia się w samym wierszu. Znak obok nazwy kontenera mówi, że od jego ostatniej kopii został odtworzony z innym obrazem, portami, zmiennymi lub wolumenami. Znika przy następnej kopii.

Trzy przyciski nad listą zawężają ją:

- **Filtry**: według pola **Harmonogram** (**Wszystkie**, **Zaplanowane**, **Wstrzymane**), według pola **Kopia zapasowa** (**Z kopią**, **Bez kopii**) oraz, dla **Containers i maszyny wirtualne**, **Zainstalowane** albo **Niezainstalowane**.
- **Sortuj**: według nazwy, stanu, IP, ostatniej kopii, rozmiaru lub harmonogramu. Każde pole poza IP sortuje w obie strony, na przykład **Rozmiar, największe najpierw** i **Rozmiar, najmniejsze najpierw**.
- **Szukaj**: według nazwy, obrazu lub IP.

Elementy, które nie są już zainstalowane, ale nadal mają kopie, stoją osobno pod **Niezainstalowany (tylko kopie zapasowe)**. Przywróć je albo usuń ich kopie, żeby zwolnić miejsce.

Główna akcja strony unosi się w prawym dolnym rogu: **Utwórz kopię wszystkiego teraz** uruchamia przebieg obejmujący każdą włączoną sekcję. **Więcej** zawiera resztę:

- **Zaznacz** zaczyna zaznaczanie, zobacz niżej.
- **Odkryj kopie zapasowe** czyta, co przechowują miejsca docelowe, i odbudowuje z tego elementy, na przykład po świeżej instalacji. Kontener bez własnych danych nie ma migawki i wraca z definicji zapisanej obok repozytorium. Kopie poza siedzibą odbudowanych sekcji czekają wtedy, dopóki nie przejmiesz ich kopii jako własnych kopii tego serwera, zobacz [Po odbudowie](offsite-recovery.md#after-a-rebuild).
- **Połączenie ZFS z tym serwerem** otwiera test SSH sekcji ZFS, zobacz [Zbiory danych ZFS](zfs-datasets.md#requirements).
- **Replikuj wszystko teraz**, przy wybranym kafelku ZFS, wysyła replikę każdego elementu ZFS.

![Ta sama lista w telefonie.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Ta sama lista w telefonie.](assets/screenshots/backups/list-phone-light.png#only-light)

## Wybieranie kilku elementów {#select}

Wiersze wybiera się tak jak pliki w menedżerze plików: Ctrl+kliknięcie, Shift+kliknięcie, Ctrl+A albo długie przytrzymanie w telefonie. Wybrany wiersz pokazuje znacznik zamiast ikony, a pasek nad listą je liczy i oferuje **Utwórz kopię zaznaczonych**, **Wstrzymaj harmonogram** i **Wznów harmonogram**, **Zaznacz wszystkie** oraz **Wyczyść zaznaczenie**. Każdy wiersz ma też menu pod prawym przyciskiem myszy z własnymi akcjami.

Zaznaczenie jest kopiowane na serwerze, element po elemencie, więc działa dalej, gdy zamkniesz kartę. Żeby przywrócić kilka elementów razem, użyj [strony Przywracanie](restore.md#several).

![Dwa wybrane kontenery z paskiem nad listą.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Dwa wybrane kontenery z paskiem nad listą.](assets/screenshots/backups/bulk-light.png#only-light)

## Dodawanie folderu lub zbioru danych ZFS {#add}

Kontenery, maszyny wirtualne i dysk flash pojawiają się same, gdy ich sekcja jest włączona w **Ustawienia, Ogólne**. Foldery i zbiory danych ZFS wybierasz sam, bo BombVault nie zgadnie, o które Ci chodzi. **Dodaj** otwiera **Co jeszcze ma kopiować BombVault?**:

- **Jeden folder** wymienia udziały, które BombVault znalazł, i przyjmuje dowolny inny folder pod Host Data. Nadaj zestawowi folderów nazwę, na przykład `Documents`. Nazwa jest stała po pierwszej kopii, bo pod nią zapisywane są migawki.
- **Zbiór danych ZFS** wymienia zbiory danych tego serwera. Wybierz ten na szczycie tego, co ma być kopiowane, zobacz [Elementy i zbiory podrzędne](zfs-datasets.md#items-and-children).
- **Konfiguracja systemu** pojawia się na hoście bez Unraid, którego Host Data sięga `/etc`. To startowy zestaw folderów `/etc`, który przeglądasz przed zapisaniem.

## Pomijanie elementu {#dont-back-up}

Element, który celowo pomijasz, na przykład testowy kontener, można oznaczyć jako **Nie kopiuj**. Przestaje wtedy liczyć się jako niechroniony, a **Nie wykluczaj już** go przywraca. **Wstrzymaj harmonogram** to łagodniejszy sposób: element opuszcza harmonogram i pełną kopię, nosi szarą plakietkę **Harmonogram wstrzymany**, a **Utwórz kopię teraz** nadal działa. Ustawia ten sam przełącznik co **W harmonogramie** na karcie Ustawienia elementu.

## Strona elementu {#entry-page}

Kliknięcie wiersza otwiera stronę elementu. Nagłówek podaje element i jego stan w jednej linii, na przykład **Ostatnia kopia 3 godziny temu** albo **Odroczono, następna próba o 03:15**, i zawiera jego akcje: **Utwórz kopię teraz**, **Przywróć**, a pod **Więcej** te rzadkie, jak **Eksportuj najnowszą kopię zapasową**, **Eksportuj kontener** albo **Eksportuj maszynę wirtualną**, **Połącz z dawnym wpisem**, **Usuń wszystkie kopie zapasowe**, **Usuń wpis** i **Usuń kopie zapasowe wszędzie**. Przycisk wstecz po lewej od ikony prowadzi do listy. Gdy kopia trwa, obok jej postępu stoi **Anuluj kopię**, a anulowany przebieg jest zapisany jako anulowany, nie jako nieudany.

**Usuń kopie zapasowe wszędzie** usuwa za jednym razem kopie zapasowe elementu ze wszystkich miejsc docelowych, łącznie z kopiami poza siedzibą. Okno wymienia każde miejsce i liczbę kopii, które w nim są, a przycisk odblokowuje się po wpisaniu nazwy elementu. Miejsce ustawione jako append-only zachowuje swoje kopie i o tym informuje. Element i jego ustawienia zostają, więc element, którego już nie kopiujesz, zostaje taki, jaki jest, a znikają tylko jego kopie zapasowe i ich historia.

Strona ma trzy karty: **Kopie zapasowe**, **Ustawienia** i **Weryfikacja**.

### Karta Kopie zapasowe {#backups-tab}

Ta karta wymienia migawki elementu, od najnowszej, z paskiem na każdy dzień. Kopia, która leży w kilku miejscach, pojawia się raz, oznaczona każdym miejscem docelowym, które ją przechowuje. Kliknięcie migawki ją przywraca, a kliknięcie miejsca wewnątrz migawki przywraca ją dokładnie stamtąd.

- **Miejsca docelowe** wymienia każde miejsce z kopiami tego elementu i ich liczbę. Miejsca poza siedzibą są czytane dopiero po naciśnięciu **Sprawdź**, bo listowanie pamięci w chmurze trwa. Miejsce, do którego element już nie kopiuje, stoi jako **Tylko starsze kopie zapasowe** i nadal można z niego przywracać.
- Każda migawka ma menu: **Dodaj tag** albo **Zmień tag**, **Porównaj z inną kopią zapasową**, które wymienia pliki, jakie doszły, zniknęły albo się zmieniły, **Eksport** jako tar, zip albo własne repozytorium restic, oraz usunięcie tylko w jednym miejscu.
- **Zrzuty bazy danych** kontenera stoją na osobnej liście, z pobieraniem, zapisem do folderu i importem.

Ile migawek zostaje, ustawia miejsce docelowe, zobacz [Jak długo przechowywać](offsite-recovery.md#keep).

![Karta Kopie zapasowe kontenera: pasek na każdy dzień i miejsca docelowe, które przechowują każdą migawkę.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Karta Kopie zapasowe kontenera: pasek na każdy dzień i miejsca docelowe, które przechowują każdą migawkę.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Karta Ustawienia {#settings-tab}

Każdy element zaczyna się od **Harmonogram i miejsce docelowe**:

- **W harmonogramie** włącza element do zaplanowanych przebiegów albo go z nich wyłącza.
- **Własny plan** daje temu jednemu elementowi plan na własny rachunek, zobacz [Własny plan dla jednego elementu](schedules.md#own-plan). Bez niego element idzie według planu swojej sekcji albo w pełnej kopii.
- Rząd przycisków, **Lokalne** i po jednym na każde miejsce docelowe poza siedzibą, mówi, dokąd trafiają kopie. **Podąża za domyślnym** znaczy, że element trafia tam, gdzie jego sekcja, a kliknięcie otwiera **Miejsca docelowe według sekcji**. Zobacz [Ustawienie dla elementu](offsite-recovery.md#placement).
- **Repozytorium** podaje, gdzie zapisywane są kopie (**Zapisane na**). To miejsce jest stałe od pierwszej kopii, a kopie można zmieniać w każdej chwili.

Karty poniżej zależą od rodzaju elementu:

| Rodzaj | Karty |
|---|---|
| Kontener | **Foldery do kopii** (które zmapowane foldery są kopiowane, z **Pomiń foldery pamięci podręcznej (CACHEDIR.TAG)**), **Wzorce wykluczeń**, **Zatrzymaj inne kontenery**, **Polecenia przed i po**, **Baza danych**, **Gdy aplikacja jest zajęta**, **Aktualizuj po udanej kopii zapasowej**, **Migawka zamiast zatrzymania**, **Dawne nazwy** |
| Maszyna wirtualna | **VM** z polami **Metoda** (**Graceful (zamknięcie)** albo **Migawka na żywo**) i **Tylko zmienione bloki**, zobacz [przewodnik po kopiach maszyn wirtualnych](vm-backup-ssh-setup.md#per-vm-method) |
| Zestaw folderów | **Folder** i **Wzorce wykluczeń** |
| Dysk flash | **Eksport zip Flash**, który zapisuje każdą kopię flash jako zwykły `.zip` w wybranym folderze, w jednym kroku, żeby narzędzie do synchronizacji nigdy nie zobaczyło połowy pliku. Kopia flash, która zawiera mniej, niż powinna, zostawia ostatni ZIP na miejscu, a przebieg eksportu o tym mówi. |
| Zbiór danych ZFS | zbiory podrzędne, kontenery do zatrzymania i polecenia wokół migawki, **Replika** i **Sprawdź dostęp do migawek**, zobacz [Zbiory danych ZFS](zfs-datasets.md) |

Ustawienia zapisują się same, gdy je zmieniasz.

![Karta Ustawienia kontenera z Własnym planem w pierwszej karcie.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Karta Ustawienia kontenera z Własnym planem w pierwszej karcie.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Karta Weryfikacja {#verify-tab}

- **Kontrole** pokazują sprawdzenie przywracania, które następuje po pierwszej kopii. **Sprawdź przywracanie** uruchamia je ponownie na najnowszej kopii: BombVault przywraca próbkę do 100 plików i 256 MiB do tymczasowego folderu i każe restic odczytać każdy plik z powrotem i porównać z jego sumami.
- **Test uruchomienia**, na kontenerze, przywraca najnowszą kopię do odizolowanej kopii i ją uruchamia: nazwa zaczynająca się od `bombvault-test-`, własna wewnętrzna sieć Docker bez opublikowanych portów i bez drogi do LAN, 1 procesor i 2 GiB pamięci. Test przechodzi, gdy kopia zgłasza się jako zdrowa, odpowiada na pierwszym wystawionym porcie albo, gdy nie ma ani jednego, ani drugiego, działa dalej. Oryginał nigdy nie jest ruszany, a kopia, jej sieć i dane są potem usuwane, także po restarcie w środku testu. Kontenery w sieci hosta, uprzywilejowane, z urządzeniami i takie, które potrzebują innego kontenera, są pokazane jako niemożliwe do przetestowania. **Test uruchomienia** w **Ustawienia, Weryfikacja, Automatyczne kontrole** testuje jeden kontener na przebieg, ten, który był testowany najdawniej.
- **Anomalie** ustawiają własną czułość i poziom powiadomień tego elementu i pokazują jego otwarte znaleziska.
- **Rozmiar według folderu** pokazuje, co zajmuje miejsce w najnowszej kopii i ile z tego przyniosła ostatnia kopia, poziom po poziomie.
- **Ostatnie uruchomienia** wymieniają ostatnie przebiegi z początkiem, końcem i czasem trwania. Kliknięcie pokazuje szczegóły.

![Karta Weryfikacja kontenera z otwartym Rozmiarem według folderu.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Karta Weryfikacja kontenera z otwartym Rozmiarem według folderu.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Wzorce wykluczeń {#excludes}

Karta **Wzorce wykluczeń** przyjmuje jeden wzorzec na linię. Wpisuj ścieżki tak, jak widać je w kontenerze, na przykład `/config/Library/Application Support/Plex Media Server/Cache`, albo samą nazwę, jak `.git`, która pasuje na dowolnej głębokości. W zestawie folderów wpisz ścieżkę tak, jak zna ją host, na przykład `/mnt/user/data/cache`. Podgląd pod polem pokazuje, na co rozwija się każda linia, i ostrzega, gdy linia nie pasuje do niczego albo wykluczyłaby wszystko. Linia `/` lub `*` albo taka, która wskazuje cały kopiowany folder, jest ignorowana, więc wykluczenie nigdy nie opróżni kopii. Żeby pominąć cały folder, odznacz go zamiast tego pod **Foldery do kopii**.

Dla znanych obrazów, takich jak Plex, Jellyfin, Sonarr, Radarr, Immich i Nextcloud, karta proponuje foldery, które aplikacja sama wypełnia na nowo, na przykład pamięci podręczne, logi i miniatury. Nic nie jest wykluczone, dopóki nie naciśniesz **Wyklucz zaznaczone**.

Nowy zestaw folderów zaczyna z włączoną grupą **Śmieci systemowe**: `.DS_Store`, `Thumbs.db`, `desktop.ini`, `@eaDir` (Synology), `.Recycle.Bin` (Unraid Recycle Bin plugin), `.Trash-*` i `*.tmp`. Każda linia to sama nazwa, więc pasuje na dowolnej głębokości pod folderem, a przed utworzeniem zestawu możesz wyłączyć dowolną z nich. Istniejący zestaw folderów zachowuje wykluczenia, które ma. **Wyklucz śmieci systemowe** w jego oknie edycji proponuje te same linie, a trafiają one do zestawu dopiero wtedy, gdy naciśniesz **Wyklucz zaznaczone** i zapiszesz.

Ta sama karta niesie ostrzeżenia, których skan folderów nie widzi. Immich na przykład trzyma albumy, twarze i daty w bazie PostgreSQL w osobnym kontenerze, więc sama kopia plików kontenera Immich przywraca zdjęcia bez tego wszystkiego, a przywrócenie wygląda na udane. Skopiuj też kontener bazy danych.

## Zrzuty baz danych {#database-dumps}

Kontenery PostgreSQL, MySQL i MariaDB, które BombVault rozpoznaje, są zrzucane przed każdą kopią, z działającego serwera, do osobnego punktu przywracania obok plików. Obejmuje to oficjalne obrazy, PostGIS, TimescaleDB, pgvector, pgautoupgrade, obrazy baz Immich, obrazy MariaDB linuxserver, yobasystems i jc21 oraz mysql-server Oracle. Kontener, który tylko wygląda jak baza danych, dostaje tę samą opcję, wyłączoną, dopóki jej nie wybierzesz.

Zrzut płynie prosto do repozytorium i nigdy nie jest zapisywany na dysku. Dane logowania pochodzą ze zmiennych samego kontenera, razem z sekretami `*_FILE`, i nigdy go nie opuszczają. Karta **Baza danych** na karcie Ustawienia kontenera mówi, czy folder danych jest zapisywany po zatrzymaniu, kopiowany w działaniu, czy niezapisywany, i włącza lub wyłącza zrzut dla tego kontenera. Etykieta `bombvault.dbdump=false` robi to samo i ma pierwszeństwo przed kartą. **Automatyczne zrzuty baz danych** w **Ustawienia, Ogólne** przełączają to dla wszystkich.

Nieudany zrzut nie psuje kopii. Pojawia się jako osobny nieudany przebieg z powodem i wysyła powiadomienie. Zrzuty nigdy nie są wczytywane z powrotem same: pobierz jeden, zwykły lub skompresowany, zapisz go do folderu albo zaimportuj do świeżo uruchomionej bazy jednym kliknięciem na karcie **Kopie zapasowe**. Żeby odzyskać zrzut bez BombVault, zobacz [Odzyskiwanie zrzutu bazy danych](offsite-recovery.md#database-dumps). Gdy jeden się nie uda, zobacz [Zrzut bazy danych się nie udał](troubleshooting.md#a-database-dump-failed).

## Polecenia i inne kontenery {#commands}

- **Polecenia przed i po** działają wewnątrz kontenera przez `sh -c`, na przykład żeby zrzucić pamięć podręczną na dysk. Polecenie przed kopią, które się nie powiedzie, przerywa kopię. Polecenie przed kopią, które kończy się kodem 75, zamiast tego odracza zaplanowaną kopię, zobacz niżej.
- **Zatrzymaj inne kontenery** wymienia kontenery, które mają być zatrzymane na czas kopii tego, na przykład jego bazę danych. Potem startują z powrotem w kolejności `depends_on` z Compose i domyślnie każdy musi zgłosić się jako zdrowy, zanim wystartuje następny. Czas oczekiwania i jego limit są pod **Ponowne uruchomienie po kopii zapasowej** na karcie **Kolejność** w **Ustawienia, Harmonogram**.
- Kontener, który dzieli przestrzeń nazw sieci, PID lub IPC z zatrzymywanym kontenerem, przez `--net=container:` albo `network_mode: service:` w Compose, zatrzymuje się i startuje razem z nim, nawet jeśli nie ma go na liście, i startuje po kontenerze, do którego dołącza. Kontener po restarcie dostaje nową sieć, a ten, który działałby dalej w starej sieci, miałby tylko swój loopback. Sama lista zostaje taka, jaką ustawiłeś. Jeśli takiego kontenera nie dało się zatrzymać, przebieg mówi, że nie ma on sieci, dopóki nie zostanie uruchomiony ponownie. To samo dotyczy własnego kontenera BombVault, który nigdy nie zatrzymuje sam siebie i nie może trafić na listę.
- Kontener, który potem nie działa ponownie, jest wymieniony w przebiegu i w powiadomieniu, także wtedy, gdy sama kopia się nie powiodła.

## Gdy aplikacja jest zajęta {#app-busy}

Kopia zatrzymuje swój kontener, więc zaplanowana może poczekać, gdy ktoś używa aplikacji, i spróbować sama jeszcze raz. Ustawia to karta **Gdy aplikacja jest zajęta** na karcie Ustawienia kontenera:

- **Sprawdzanie aktywności** pyta Plex, Jellyfin lub Emby przed zaplanowaną kopią, czy ktoś ogląda. Plex dostaje pytanie z tokenem z własnego `Preferences.xml`, więc nic nie trzeba wpisywać. Jellyfin i Emby przyjmują **Klucz API**, utworzony w panelu serwera w kluczach API.
- **Ignoruj wstrzymane sesje po (min)** pozwala, by film zatrzymany na noc przestał się liczyć.
- **Twórz kopię, gdy serwer nie odpowiada** decyduje, co się dzieje, gdy nie da się zapytać serwera: włączone, kopia się wykonuje, a dziennik to mówi. Wyłączone, czeka tak, jakby ktoś oglądał.
- **Czekaj, gdy kontener wysyła dane** jest dla aplikacji bez takiej kontroli, jak serwer gier czy program do pobierania. Kopia czeka, gdy kontener wysłał więcej niż **Zajęty od (KiB/s)** przy dowolnym odczycie w ciągu ostatnich **Spokój przez (min)** minut.
- **Ponawiaj co (min)**, **Próbuj tylko w oknie czasowym** z **Próbuj między** i **Czekaj najwyżej (godz.)** ustawiają przebieg prób.
- **Na koniec oczekiwania** albo się poddaje, co pokazuje kopię pod **Wymaga uwagi** z **Spróbuj ponownie**, albo **Utwórz kopię mimo to**.

Oczekująca kopia jest zapisana jako **Odroczono**, na bursztynowo, a reszta przebiegu idzie dalej. Przegląd, lista i element pokazują ją z następną próbą, a alert o spóźnieniu i stan ochrony czekają na nią do jej terminu. Dostajesz jedno powiadomienie, gdy kopia jest odroczona, i jedno, gdy się kończy, a nie jedno na każdą próbę. Próba czeka, gdy trwa inna kopia, a oczekiwanie trwa dalej po restarcie. Kopia, którą uruchamiasz sam, zawsze rusza od razu, czy to przez **Utwórz kopię teraz**, **Spróbuj ponownie**, zaznaczenie, API czy MCP, a ta, która przejdzie, kończy oczekiwanie w toku.

Karta Ustawienia kontenera ma też starszy, prostszy sposób: **Czekaj, aż aplikacja będzie bezczynna** z **Czekaj najwyżej (godz.)**. Kopia startuje, gdy tylko aplikacja jest bezczynna, co określa karta **Bezczynność przed kopią** w **Ustawienia, Harmonogram**. Członkowie stosu Compose, którzy mają kopię w tym samym przebiegu, czekają razem, a oczekująca kopia nie trzyma blokady, więc pozostałe kontenery idą dalej.

![Karta Gdy aplikacja jest zajęta kontenera Plex z ustawionym sprawdzaniem aktywności.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Karta Gdy aplikacja jest zajęta kontenera Plex z ustawionym sprawdzaniem aktywności.](assets/screenshots/backups/app-busy-light.png#only-light)

### Przykład: Plex czeka, gdy ktoś ogląda {#example-plex}

Pełna kopia rusza co noc o 03:00. Nikt nie powinien zobaczyć, jak film się zatrzymuje, bo Plex padł na czas kopii, ale kopia i tak ma się wykonać przed rankiem.

1. Otwórz **Kopie zapasowe**, potem element `plex`, a potem jego kartę **Ustawienia**.
2. W **Gdy aplikacja jest zajęta** ustaw **Sprawdzanie aktywności** na Plex. Nic nie trzeba wpisywać.
3. Ustaw **Ignoruj wstrzymane sesje po (min)** na `30`, żeby film wstrzymany o północy przestał się liczyć o wpół do pierwszej.
4. Zostaw włączone **Twórz kopię, gdy serwer nie odpowiada**.
5. Ustaw **Ponawiaj co (min)** na `15`.
6. Włącz **Próbuj tylko w oknie czasowym** i ustaw **Próbuj między** na `22:00` i `06:00`.
7. Ustaw **Czekaj najwyżej (godz.)** na `3`, a **Na koniec oczekiwania** na **Utwórz kopię mimo to**.

Co się dzieje w nocy, gdy ktoś jeszcze ogląda o 03:00: BombVault pyta Plex, znajduje sesję, która gra, i zapisuje kopię jako **Odroczono** z następną próbą o 03:15. Pozostałe kontenery przebiegu są kopiowane jak zwykle. Gdy film kończy się o 04:10, próba o 04:15 zastaje Plex bezczynny, kopiuje go i oczekiwanie kończy się jednym powiadomieniem. Gdyby film nadal trwał o 06:00, oczekiwanie doszłoby do końca i Plex zostałby skopiowany mimo to, więc jego kopie nigdy nie spóźniają się o więcej niż trzy godziny.

Do własnej kontroli spraw, by **Polecenie przed kopią** kończyło się kodem 75, gdy aplikacja jest zajęta, na przykład `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Więcej objawów i rozwiązań jest w [Rozwiązywaniu problemów](troubleshooting.md#deferred-backups).

## Zwykłe eksporty {#exports}

restic pozostaje silnikiem, ale eksport daje kopię, która nie potrzebuje ani BombVault, ani restic.

- **Eksportuj kontener** i **Eksportuj maszynę wirtualną** pod **Więcej** na stronie elementu zapisują kopię taką, jaka jest w tej chwili, obok repozytorium: foldery kontenera do kopii wraz z jego szablonem Unraid albo obrazy dysków maszyny wraz z jej definicją, do przywrócenia przez `virsh define`.
- **Eksport** na migawce zapisuje zapisaną kopię do folderu eksportu na serwerze, jako tar, zip albo własne repozytorium restic, i może objąć jedną migawkę albo wszystkie. Kopia flash i autokopia zawierają klucze, więc ich eksport wymaga hasła logowania i zalogowania na samej instancji.

Eksporty są domyślnie zwykłymi plikami. **Szyfruj eksporty za pomocą age** w **Ustawienia, System** zabezpiecza je, razem z ZIP-em flash, dla wymienionych odbiorców, z których każdy to klucz publiczny age albo klucz publiczny SSH. Odszyfrujesz je później poza serwerem pasującym kluczem prywatnym. Gdy szyfrowanie jest włączone, a nie ma ważnego odbiorcy, eksport kończy się wyraźnym błędem, zamiast kiedykolwiek zapisać zwykłe pliki.

## Co mówi o sobie przebieg {#run-notes}

- **Czas.** Każdy przebieg podaje początek, koniec i czas trwania, a każdy element ma własne **Ostatnie uruchomienia**.
- **Dlaczego kopia była wolna.** Gdy kopia trwa, BombVault obserwuje, jak zajęte są procesor, dyski i sieć. Gdy kopia trwa dużo dłużej niż zwykle, a jedna rzecz wyraźnie była u swojej granicy, przebieg to mówi, na przykład „Dysk docelowy disk1 był zajęty w 98%”. W przeciwnym razie milczy.
- **Postęp kontroli i przycinania.** Dziennik aktywności i karta **Sprawdź kopie zapasowe** pokazują, jak daleko restic doliczył, na przykład *12 z 47 paczek*, z czasem do końca tego kroku, gdy jest dość danych, by go oszacować.
- **Pliki, których nie dało się odczytać.** Kopia, która przeszła, ale coś pominęła, robi swoją linię pomarańczową i wymienia, co pominięto.
