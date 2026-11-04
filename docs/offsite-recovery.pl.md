# Kopie poza siedzibą i odzyskiwanie

!!! note "Kopie poza siedzibą czekają po odbudowie"
    Gdy Kreator awaryjny lub **Odkryj kopie zapasowe** odbudowuje elementy bez dawnych ustawień, kopie poza siedzibą tych sekcji są wstrzymane, dopóki ich ustawienie domyślne nie zostanie potwierdzone. Zobacz [Po odbudowie](#after-a-rebuild).

Lokalne kopie zapasowe chronią przed utraconym kontenerem lub złą aktualizacją. Kopie poza siedzibą i przetestowany zestaw odzyskiwania chronią przed utratą całej maszyny, ransomware albo pożarem. Ta strona omawia miejsca docelowe i kopie poza siedzibą, zabezpieczanie kopii przed manipulacją, dowodzenie, że potrafisz przywrócić dane, parowanie kilku serwerów i odzyskanie wszystkiego, gdy samego BombVault już nie ma.

## Kopie poza siedzibą {#off-site-copies}

Zachowaj szybką kopię lokalną i dodaj jedno lub więcej miejsc docelowych poza domem. BombVault kopiuje tam nowe migawki poleceniem `restic copy` w trybie best-effort, więc potknięcie poza siedzibą nigdy nie powoduje niepowodzenia kopii lokalnej. W tym układzie każda sekcja zapisuje najpierw do swojego własnego folderu, a repozytorium poza siedzibą jest kopią. Własny folder nie musi leżeć na tym serwerze: zobacz [Własny folder sekcji](#own-folder), jeśli chcesz trzymać go w miejscu docelowym takim jak S3 czy rest-server.

- **Kilka miejsc docelowych na sekcję.** Każda sekcja (kontenery, maszyny wirtualne, flash, autokopia, foldery i zbiory danych ZFS) może kopiować naraz do kilku miejsc docelowych poza siedzibą, więc możesz trzymać na przykład rest-server u znajomego i koszyk S3 równolegle. Karta **Miejsca docelowe według sekcji** na stronie Miejsca docelowe włącza lub wyłącza każdą linię od sekcji do miejsca docelowego.
- **Kiedy działają kopie.** Karta **Kopiuj** miejsca docelowego ustawia **Kiedy kopiować**: **Po każdej kopii zapasowej** albo według własnego planu, który zbiera migawki i kopiuje w ustalonych porach. W **Ustawienia, Harmonogram, Kopie poza siedzibą według sekcji** **Wyjątek** daje jednej sekcji własny plan kopiowania.
- **Karta Kopie poza siedzibą.** W **Ustawienia, Miejsca docelowe** każda sekcja z kopią poza siedzibą ma wiersz w **Kopie poza siedzibą**, z **Testuj połączenie**, **Replikuj teraz** do uruchomienia kopiowania na żądanie oraz **Skonfiguruj** lub **Edytuj**. Reszta ustawień kopiowania sekcji otwiera się stamtąd w jej oknie.
- **Przechowywanie** ustawia się osobno dla każdego miejsca docelowego w **Jak długo przechowywać**, więc kopie poza siedzibą możesz trzymać dłużej jako archiwum. Kopia w polu sekcji poza siedzibą podąża za nim jak każda inna, a sekcja może mieć własne wartości w **Reguły według sekcji**. Same zera nigdy nie przycinają migawek poza siedzibą. Zobacz [Jak długo przechowywać](#keep).
- **Limity przepustowości** (**Przepustowość poza siedzibą** w **Ustawienia, Miejsca docelowe** albo karta **Kopiuj** miejsca docelowego) ograniczają tempo wysyłania i pobierania restic, żeby kopie nie nasycały Twojego WAN. Wpisuje się je w KiB/s, czyli w jednostce, którą przyjmuje restic, a każde pole pokazuje to samo tempo w MB/s i Mbit/s podczas pisania: 2048 czyta się jako około 2,0 MB/s (16,8 Mbit/s).
- **Kompresja** to **Wyłączone**, **Automatycznie** (domyślne w restic) albo **Maksymalnie**, ustawiana w każdym miejscu docelowym i, dla jednej sekcji, w jej regułach tam. Kopie, kopiowanie i przycinanie zapisują z nią, a zestaw odzyskiwania ją podaje, więc zwykły restic może pisać dalej tak samo.
- **Najpierw streaming.** Gdy serwer multimediów, taki jak Plex, Jellyfin czy Emby, streamuje, kopie wysyłają z niższym limitem i wracają do zwykłego kilka minut po końcu streamu. Karta **Streaming** na stronie Miejsca docelowe ma **Spowalniaj kopie poza siedzibą podczas streamingu**, **Serwery multimediów** (kontenery z Plex, Jellyfin lub Emby w nazwie obrazu, dopóki nie wybierzesz), **Streaming powyżej (Mbit/s)**, **Limit wysyłania podczas streamingu (KiB/s)** i **Powrót do normy po (min)**. BombVault odczytuje ruch z Dockera, więc serwera multimediów w sieci hosta nie da się zmierzyć. REST, S3, B2, Azure, Google Cloud, Swift i rclone przez HTTP zwalniają w środku kopiowania. SFTP oraz foldery lokalne lub zamontowane przyjmują niższy limit przy następnym kroku kopiowania.
- **Zimna pamięć (S3).** Miejsce docelowe S3 może używać tańszej **Klasa magazynu**, którą restic nadal odczyta od razu, na przykład Standard-IA, One Zone-IA, Intelligent-Tiering albo Glacier Instant Retrieval. Glacier Flexible Retrieval i Deep Archive są pominięte, bo restic nie potrafi z nich przywracać. Zasób rclone ustawia swoją klasę w konfiguracji rclone.
- **Budżet wzrostu.** **Budżet wzrostu (GB, 0 = wył.)** w oknie kopiowania sekcji podnosi alarm, gdy repozytorium przekroczy ten rozmiar. Niczego nie usuwa, więc pasuje do kopii append-only, której stąd nic nie przycina.
- Gdy kopiowanie trwa, nagłówek stanu przeglądu pokazuje **Trwa kopiowanie poza siedzibę**. To aktywny wskaźnik, a nie pasek procentowy, bo `restic copy` nie udostępnia postępu w formie czytelnej dla maszyn.

!!! note "Przywracanie z dowolnego miejsca"
    Każdy element pokazuje swoje kopie jako jedną oś czasu we wszystkich miejscach, gdzie leży kopia, na karcie **Kopie zapasowe**. Kopia skopiowana do B2 pojawia się raz, oznaczona każdym miejscem, które ją przechowuje. Przywracanie bierze pierwsze miejsce, do którego sięga, zaczynając od repozytorium, do którego element jest zapisywany, a dla każdej migawki możesz wskazać inne. Miejsca docelowe poza siedzibą są odczytywane dopiero po kliknięciu **Sprawdź**, bo listowanie pamięci w chmurze trwa. Usuwanie w jednym miejscu najpierw sprawdza pozostałe i mówi, czy to była ostatnia kopia. Zobacz [Przywracanie](restore.md).

## Miejsca docelowe {#destinations}

Miejsce docelowe to miejsce, do którego trafiają kopie zapasowe: folder lokalny, udział, własny serwer albo usługa w chmurze. Ustawia się je raz i obsługuje potem każdą sekcję. Otwórz **Miejsca docelowe** z kafelka ustawień albo włącz tam **Pokaż także na pasku bocznym**, aby je przypiąć.

![Strona Miejsca docelowe: lista miejsc i przepływ pokazujący, która sekcja dokąd trafia.](assets/screenshots/storage/locations-dark.png#only-dark)
![Strona Miejsca docelowe: lista miejsc i przepływ pokazujący, która sekcja dokąd trafia.](assets/screenshots/storage/locations-light.png#only-light)

**Dodaj miejsce docelowe** otwiera okno w czterech krokach:

1. **Dostawca.** Każda usługa ma swoje logo, w prawym górnym rogu okna jest wyszukiwarka, a usługi są pogrupowane: **Lokalne** dla dysku lub udziału podłączonego do tego serwera, **NAS lub serwer** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV i inne), **Usługa S3** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 i inne), własny serwer S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Chmura** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud i reszta obsługiwana przez rclone) oraz **Z twojej grupy**, czyli serwery odbiorcze Twojej grupy. Każda mówi, jak dobrze nadaje się do kopii zapasowych: dyski w chmurze zwalniają przy wielu żądaniach, więc pierwsza kopia i przycinanie trwają tam dłużej. Dla usługi, której żaden kafelek nie obejmuje, kafelek **Rclone** w **Własna konfiguracja rclone** przyjmuje zasoby wklejone z rclone.conf.
2. **Logowanie.** Pola zależą od usługi: klucz dostępu dla S3, użytkownik i hasło dla WebDAV i SMB, hasło aplikacji tam, gdzie logowanie dwuskładnikowe blokuje zwykłe, publiczny klucz SSH BombVault dla SFTP i Storage Box albo token dla usług, które logują się przez przeglądarkę. Dla nich okno pokazuje polecenie `rclone authorize` do uruchomienia na komputerze z przeglądarką, a wypisany przez nie token trafia do pola. Gdy masz już [zapisane dane logowania](#sign-ins) do tej usługi, **Zapisane dane logowania** wybiera je zamiast **Nowe klucze**. **Testuj połączenie** sprawdza logowanie, zanim cokolwiek zostanie zapisane. Okno wyświetla potem foldery w miejscu docelowym, z **Nowy folder** do utworzenia jednego i wolnym miejscem tam, gdzie usługa je zgłasza. Pusty folder jest najbezpieczniejszy. BombVault umieszcza w nim jeden folder na sekcję.
3. **Ochrona.** Okno mówi wprost, co usługa potrafi. Rest-server w trybie append-only odmawia usuwania, a test manipulacji to sprawdza. Koszyk S3 może zachowywać stare wersje dzięki wersjonowaniu i blokadzie obiektów, czego BombVault jeszcze nie umie sprawdzić. Dysk w chmurze nie potrafi odmówić usunięcia wcale: kto wejdzie na serwer, wejdzie też do tej kopii. Włącz **Ochrona przed usunięciem (append-only)** tylko tam, gdzie druga strona naprawdę odmawia usuwania. BombVault nigdy wtedy tam nie przycina. **Na wypadek awarii** mówi, co zestaw odzyskiwania zawiera dla tego miejsca.
4. **Dla których sekcji.** Wybierz sekcje, które tworzą tu kopie. Pierwsze kopiowanie zabiera całą historię. Elementy, które wybrały własne miejsca docelowe, nie dostają nowego same z siebie, a okno je wymienia.

![Dodaj miejsce docelowe: pierwszy krok, z każdym dostawcą jako kafelkiem.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Dodaj miejsce docelowe: pierwszy krok, z każdym dostawcą jako kafelkiem.](assets/screenshots/storage/add-providers-light.png#only-light)

![Dodaj miejsce docelowe: krok logowania po udanym teście.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Dodaj miejsce docelowe: krok logowania po udanym teście.](assets/screenshots/storage/add-signin-light.png#only-light)

Niektóre usługi przy logowaniu proszą o trochę więcej:

- **SFTP i Storage Box.** **Dodaj klucz** umieszcza publiczny klucz BombVault na serwerze za pomocą hasła z pola **Hasło konta**, które służy tylko do tego jednego logowania i nie jest nigdzie zapisywane. Pod spodem widać polecenie, którym można to zrobić ręcznie. Test pokazuje **Klucz serwera** z jego odciskiem. BombVault go zapamiętuje i odrzuca serwer, jeśli później pokaże inny.
- **Klucze S3 dla pojedynczego koszyka**, takie jak token obiektowy Cloudflare R2 albo ograniczony klucz AWS, mogą nie mieć prawa wyświetlać innych koszyków. Wpisz koszyk w pole **Bucket**, a test i lista folderów otworzą właśnie ten koszyk.
- **Regiony i adresy S3.** Region jest odczytywany z endpointu, który go podaje (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage zaczyna od własnego regionu domyślnego, a IDrive e2 prosi o endpoint, który pokazuje jego panel. Adres wklejony razem z koszykiem, tak jak pokazuje go panel R2, wypełnia serwer i koszyk. Koszyk utworzony z przeglądarki folderów trafia do regionu, który podaje formularz.
- **Klasa magazynu.** Lista oferuje tylko klasy, które usługa przyjmuje, a **Więcej ustawień** pokazuje tylko opcje, które kształtują nowy koszyk.
- **Google Drive i OneDrive.** Przycięte dane są usuwane na stałe, zamiast trafiać do kosza, gdzie nadal liczyłyby się do limitu. Dysk współdzielony Google znajduje się w **Więcej ustawień**.
- **Serwery SMB, które nie wyświetlają swoich udziałów,** dostają w polu **Udział** nazwę jednego z nich, a test otwiera ten udział bezpośrednio.
- **FTP** pyta o **Szyfrowanie**: **Brak**, **Jawne TLS (FTPES)** albo **Niejawne TLS (FTPS, port 990)**.
- **OneDrive i Zoho WorkDrive.** Gdy logowanie otwiera kilka dysków lub obszarów roboczych, okno pyta, na który mają trafiać kopie zapasowe.
- **rest-server z certyfikatem samopodpisanym.** Umieść certyfikat jako plik PEM w folderze `certs` w folderze danych BombVault (`/config/certs` w kontenerze, `appdata/bombvault/certs` w Unraid). Kopie zapasowe, kopie i test ochrony przed usunięciem ufają mu bez restartu.

Usługi S3 działają przez własny backend S3 restic, dzięki czemu mogą obowiązywać klasa magazynu i blokada obiektów. Do rest-servera restic sięga bezpośrednio. Każda inna usługa działa przez rclone dołączone do BombVault, a jej część konfiguracji rclone widać w karcie **rclone.conf** na stronie miejsca, z ukrytymi sekretami. Eksport ustawień zawiera miejsca docelowe, a z dołączonymi danymi logowania zawiera także ich logowanie.

Zapisanie miejsca docelowego, które już przechowuje kopie kontenerów, maszyn wirtualnych lub folderów, na przykład takiego, które usunąłeś i dodajesz ponownie, dodaje je do tych sekcji jako niezaznaczoną kopię. Nic nie jest tam kopiowane, dopóki go nie zaznaczysz, ale **Przywróć** od razu oferuje stare kopie.

### Strona miejsca docelowego {#location-page}

Każde miejsce docelowe ma własną stronę. Pokazuje tylko te karty, które go dotyczą, w tej kolejności:

![Strona miejsca docelowego: Własne foldery, Połączenie i Dane logowania.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Strona miejsca docelowego: Własne foldery, Połączenie i Dane logowania.](assets/screenshots/storage/location-page-light.png#only-light)

- **Własne foldery**: sekcje, które zapisują tu swoje kopie najpierw. Logowanie, ochrona i limity tego miejsca ich dotyczą. **Użyj jako własnego folderu** przenosi tu własny folder kolejnych sekcji, a **Zmień** przenosi jeden z powrotem gdzie indziej. Zobacz [Własny folder sekcji](#own-folder).
- **Kopie trafiają prosto tutaj**: każda sekcja, której elementy kopiują prosto w to miejsce zamiast do własnego folderu, i ile elementów tak robi. Takie repozytorium bezpośrednie bierze wszystkie ustawienia z tego miejsca. To, do którego już nic nie kopiuje, mówi o tym, a **Usuń** zdejmuje je z listy. Kopie w jego miejscu zostają. Zobacz [Repozytoria bezpośrednie](#direct-repositories).
- **Połączenie**: **Nazwa**, przełącznik **Dostępne** (wyłączone znaczy, że BombVault już tu nie zapisuje, a elementy kopiujące tu kończą się czytelnym komunikatem zamiast trafiać gdzie indziej), folder, **Kompresja**, **Klasa magazynu** i **Poza lokalizacją**. W miejscu **Lokalne** wymienia folder każdej sekcji w **Własne foldery tutaj**. **Zmień połączenie** testuje i zapisuje nowy adres lub logowanie. Gdy inne miejsca logują się tymi samymi zapisanymi danymi, okno o tym mówi, a zmiana tutaj daje temu miejscu własne dane logowania zamiast przepisywać wspólne.
- **Dane logowania**: z których zapisanych danych korzysta (**Loguje się przez**), które inne miejsca używają ich też (**Używane też przez**) oraz **Edytuj**, co otwiera okno logowania. Zobacz [Dane logowania](#sign-ins).
- **rclone.conf**: część konfiguracji rclone tego miejsca, dla miejsca działającego przez rclone.
- **Ochrona przed usunięciem**, z **Ochrona przed usunięciem (append-only)**, **Akceptuj bez ochrony przed usunięciem**, **Przetestuj ochronę przed usunięciem teraz** i **Kto przycina**, co mówi, czy BombVault przycina tu kopię i własne foldery, czy zostawia to drugiej stronie. Folder lokalny pokazuje tę kartę tylko wtedy, gdy ma ochronę, którą można wyłączyć. Zobacz [Ochrona przed usunięciem](#delete-protection-append-only).
- **Kopiuj**: **Kiedy kopiować** oraz **Limit wysyłania (KiB/s)** i **Limit pobierania (KiB/s)**. Miejsce, które ma tylko własne foldery i żadnych kopii, pokazuje limity jako kartę **Przepustowość**.
- **Jak długo przechowywać**: **Krótko**, **Zrównoważone**, **Długo** albo **Własne wartości**, z osią czasu pokazującą, które kopie reguła zachowuje. Zobacz [Jak długo przechowywać](#keep).
- **Już pod tym miejscem docelowym**: sekcje, których wpisany ręcznie adres leży w folderze tego miejsca. Przejęcie jednej zostawia repozytorium tam, gdzie jest. Logowanie, klasa magazynu i ochrona przed usunięciem pochodzą wtedy z miejsca. BombVault najpierw sprawdza, czy logowanie miejsca otwiera repozytorium, i odmawia umieszczenia repozytorium append-only pod miejscem, które nie jest append-only.
- **Reguły według sekcji**: własne repozytorium, przechowywanie lub kompresja jednej sekcji w tym miejscu. **Zmień reguły** je otwiera, a **Przywróć ustawienia miejsca docelowego** przywraca sekcję. Logowanie, przepustowość i pora kopiowania zawsze pochodzą z miejsca.
- Naczynie pokazujące, jak pełne jest miejsce, ze znacznikiem, gdzie jest pełne, i liczbą tygodni, które zostały przy przyroście z ostatnich tygodni. Miejsce, które nie zgłasza rozmiaru, mówi **Rozmiar nieznany**.

**Przenieś** i **Usuń miejsce docelowe** stoją wśród akcji strony i są opisane w [Przenoszenie i usuwanie](#move-remove).

Zdalne miejsce docelowe, które miało kopie i nagle nie ma repozytorium, na przykład rest-server, którego dysk się nie zamontował, nie dostaje samo z siebie nowego, pustego. Jego strona mówi o tym, gdy tylko kopia lub kopiowanie zauważy brak repozytorium. Zamontuj dysk ponownie. Jeśli celowo opróżniłeś stare repozytorium, **Utwórz tu nowe repozytorium** zaczyna od nowa, dla wszystkich sekcji tego miejsca i jego repozytoriów naraz.

Serwer odbiorczy, który uruchamia inna instancja Twojej grupy, pojawia się wśród dostawców jako **Z twojej grupy**. Zobacz [Serwer odbiorczy](#receiving-server).

BombVault sprawdza swoje miejsca docelowe przy każdym starcie. Naprawia to, co nie wymaga Twojej decyzji, na przykład link do kopii lub repozytorium, których już nie ma, kopię na własnym folderze sekcji albo repozytorium bezpośrednie, którego kopii już nie ma, i zapisuje w dzienniku linię za każdą poprawkę. To, czego nie da się bezpiecznie naprawić, jest tylko zapisywane.

![Ta sama strona w telefonie.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Ta sama strona w telefonie.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Jak długo przechowywać {#keep}

Jak długo kopie są przechowywane, ustawia się dla każdego miejsca docelowego, w jego karcie **Jak długo przechowywać**. Reguła obejmuje kopie w tym miejscu i leżące tam własne foldery, także w miejscu **Lokalne**. Sekcja może mieć w jednym miejscu własne wartości w **Reguły według sekcji**, a te wygrywają z kartą miejsca dla tej sekcji.

Reguła zachowuje jedną kopię na dzień, tydzień, miesiąc i rok, tak daleko wstecz, jak ustawisz, a resztę usuwa po każdej kopii lub kopiowaniu:

| Ustawienie | Najnowsze kopie | Dni | Tygodnie | Miesiące | Lata |
|---|---|---|---|---|---|
| **Krótko** | 0 | 7 | 4 | 3 | 0 |
| **Zrównoważone** | 0 | 7 | 4 | 6 | 1 |
| **Długo** | 0 | 14 | 8 | 12 | 3 |

**Własne wartości** pozwalają ustawić pięć liczb samodzielnie. 0 wyłącza daną liczbę, a reguła ze wszystkimi zerami zachowuje wszystko. Kropki na karcie pokazują, które z 40 przykładowych kopii reguła zachowuje, z najnowszą po prawej. Przykład jest taki sam dla każdego miejsca, bo obrazek powstaje, zanim BombVault przeczyta tamtejsze kopie.

**Pytaj przed zachowaniem mniej**, domyślnie włączone, pyta przed zapisaniem zmiany, która zachowuje mniej kopii, i mówi, ile z 40 przykładowych kopii nowa reguła usuwa. Dotyczy każdego miejsca docelowego. **Przywróć ustawienia miejsca docelowego** w **Reguły według sekcji** zadaje to samo pytanie.

Miejsce z ochroną przed usunięciem nigdy nie jest przycinane z tej maszyny. Jego druga strona zachowuje lub porzuca stare kopie według własnych reguł. Dopóki element ma otwartą krytyczną anomalię o utraconych danych, przechowywanie zachowuje jego stare kopie, aż ją potwierdzisz. Zobacz [Anomalie](anomalies.md).

Aby zobaczyć, co następne uruchomienie naprawdę usunie, otwórz **Podgląd** w **Ustawienia, Miejsca docelowe**. Wymienia **Co usunęłoby następne uruchomienie**, dla każdego repozytorium i elementu, z nazwanymi punktami przywracania. Nie zakłada blokady i niczego nie zmienia.

![Jak długo przechowywać z wybranym Zrównoważone: kropki pokazują, które z 40 przykładowych kopii zostają.](assets/screenshots/storage/keep-dark.png#only-dark)
![Jak długo przechowywać z wybranym Zrównoważone: kropki pokazują, które z 40 przykładowych kopii zostają.](assets/screenshots/storage/keep-light.png#only-light)

#### Przykład: co zachowuje Zrównoważone {#example-retention}

Kontener jest kopiowany co noc o 03:00 od 1 października 2025, więc w środę 4 listopada 2026 ma 400 kopii. Jego miejsce docelowe jest ustawione na **Zrównoważone**: 7 dni, 4 tygodnie, 6 miesięcy i 1 rok.

Każda liczba zachowuje najnowszą kopię swojego dnia, tygodnia, miesiąca lub roku, a kopia wybrana przez kilka liczb jest zachowana raz. Tygodnie biegną od poniedziałku do niedzieli:

| Reguła | Kopie, które wybiera | Nowe, które dodaje |
|---|---|---|
| 7 dni | 4 lis, 3 lis, 2 lis, 1 lis, 31 paź, 30 paź, 29 paź | wszystkie 7 |
| 4 tygodnie | 4 lis, 1 lis, 25 paź, 18 paź | 25 paź, 18 paź |
| 6 miesięcy | 4 lis, 31 paź, 30 wrz, 31 sie, 31 lip, 30 cze | 30 wrz, 31 sie, 31 lip, 30 cze |
| 1 rok | 4 lis | żadnej |

Zostaje 13 kopii z 400. Najstarsza pochodzi z 30 czerwca, więc możesz cofnąć się nieco ponad cztery miesiące. Dzięki deduplikacji zajmują niewiele więcej miejsca niż jedna pełna kopia plus to, co zmieniło się po drodze.

**Lata** zasługują na drugie spojrzenie. Liczą lata kalendarzowe, w których jest kopia, zaczynając od bieżącego, a najnowszą kopią tego roku jest dzisiejsza. Dlatego 1 w **Lata** nie dodaje niczego do pozostałych liczb i potrzebujesz 2, żeby sięgnąć do zeszłego roku. Te same 400 kopii przy pozostałych ustawieniach:

| Ustawienie | Zachowane kopie | Najstarsza kopia |
|---|---|---|
| **Krótko** | 10 | 30 września 2026 |
| **Zrównoważone** | 13 | 30 czerwca 2026 |
| **Długo** | 29 | 31 grudnia 2025 |

Przejście ze **Zrównoważone** na **Krótko** najpierw pyta, z powodu **Pytaj przed zachowaniem mniej**: „Z 40 kopii na obrazku nowa reguła usuwa 3, które obecna zachowuje.” Liczba pochodzi z przykładu na obrazku, a nie z Twoich własnych kopii. Tu akurat się zgadza: zniknęłyby 31 sierpnia, 31 lipca i 30 czerwca. **Zapisz mimo to** stosuje regułę, a następne przycinanie usuwa je z tego miejsca na stałe.

Aby w tym miejscu trzymać kontenery dłużej niż resztę, naciśnij **Zmień reguły** w **Reguły według sekcji**, wybierz **Containers** i ustaw **Własne wartości** w **Reguły dla Containers**, na przykład 14 dni, 8 tygodni, 12 miesięcy i 2 lata. Pozostałe sekcje nadal podążają za kartą miejsca, a **Przywróć ustawienia miejsca docelowego** przywraca Containers.

### Dane logowania {#sign-ins}

Dane logowania to klucz albo hasło, którego BombVault używa dla usługi S3 lub rest-servera. Są jedną listą, kartą **Dane logowania** w **Ustawienia, Miejsca docelowe**, a każdy wiersz mówi, które miejsca docelowe z nich korzystają (**Używane przez**).

- **Zapisane przy dodawaniu miejsca.** Dodaj miejsce docelowe zapisuje wpisane klucze jako dane logowania. Następne miejsce w tej samej usłudze może je wybrać w **Zapisane dane logowania** zamiast **Nowe klucze**, a późniejsza zmiana dotyczy każdego miejsca, które ich używa.
- **Domyślne.** Gwiazdka oznacza domyślne dane logowania. Są używane wszędzie tam, gdzie nie wybrano żadnych, na przykład przy adresie wpisanym ręcznie, źródle pobierania albo odzyskiwaniu na świeżej instalacji. **Ustaw jako domyślne** daje gwiazdkę innym.
- **Edytuj i Testuj.** **Edytuj**, tutaj lub w karcie **Dane logowania** miejsca, otwiera jedno okno dla danych logowania. **Testuj** loguje się do każdego miejsca, które ich używa, tym, co wpisałeś, zanim cokolwiek zostanie zapisane. **Zapisz** je zachowuje.
- **Usuwanie.** Dane logowania można usunąć tylko wtedy, gdy nie używa ich żadne miejsce docelowe ani źródło pobierania, a domyślne odchodzą dopiero jako ostatnie. Wiersz mówi wtedy, co jeszcze z nich korzysta.
- **Zmiana połączenia miejsca** nigdy nie przepisuje danych logowania, które dzielą inne miejsca. Miejsce dostaje zamiast tego własne, a okno mówi wcześniej, że jego dane logowania są wspólne. Aby zmienić wspólne wszędzie, edytuj je w karcie **Dane logowania**.

![Karta Dane logowania: każde zapisane logowanie z tym, co z niego korzysta, i gwiazdka na domyślnym.](assets/screenshots/storage/signins-dark.png#only-dark)
![Karta Dane logowania: każde zapisane logowanie z tym, co z niego korzysta, i gwiazdka na domyślnym.](assets/screenshots/storage/signins-light.png#only-light)

Folder na tym serwerze nie potrzebuje danych logowania. SMB, WebDAV, SFTP, Storage Box i dyski w chmurze trzymają je w konfiguracji rclone, którą pokazuje karta **rclone.conf** miejsca, więc nie widać ich na tej liście.

### SMB, WebDAV i inne zasoby rclone {#smb-webdav}

Udział Windows lub Samba i serwer WebDAV (Nextcloud, ownCloud, SharePoint lub każdy inny) to kafelki dostawców w **Dodaj miejsce docelowe**, w grupie **NAS lub serwer**. Wpisz host i udział (SMB) albo adres i typ serwera (WebDAV), użytkownika i hasło, a BombVault napisze za Ciebie sekcję rclone. rclone zaciemnia hasło, zanim zostanie zapisane.

Dla usługi, której żaden kafelek nie obejmuje, kafelek **Rclone** w **Własna konfiguracja rclone** przyjmuje zasoby wklejone z rclone.conf. Są dodawane do zasobów, które BombVault już ma, i nigdy ich nie zastępują, a przechowywane są zaszyfrowane. Aby kopiować sekcję do jednego z nich, wpisz `rclone:<remote>:<folder>` jako repozytorium poza siedzibą tej sekcji, na przykład `rclone:nas:backups/bombvault`. Udział lub koszyk to pierwszy segment ścieżki, a nie część nazwy.

To lepsza droga niż montowanie udziału na Unraid: restic odradza trzymanie repozytorium na zamontowanym udziale CIFS, a tutaj nic nie jest montowane. NFS nie ma kafelka, bo ani restic, ani rclone nie mają backendu NFS. Dla NFS zamontuj eksport na hoście i użyj go jako folderu lokalnego.

### Przenoszenie i usuwanie {#move-remove}

- **Przenieś** zabiera kopię jednej sekcji do innego miejsca docelowego, z całą historią. Wszystko zostaje w starym miejscu. BombVault niczego tam nie usuwa. Przenoszenie działa w tle i może trwać długo: około 9 minut na 100 GB między dyskami lokalnymi, godziny przez wolne łącze. Przegląd pokazuje je, gdy trwa.
- **Usuń miejsce docelowe** oferuje dwa sposoby. **Usuń miejsce docelowe, zachowaj kopie** przestaje tam zapisywać i zostawia kopie na miejscu. Możesz je później odczytać w **Przywracanie, Ze starego folderu z kopiami**. **Usuń miejsce docelowe i skasuj tamtejsze kopie zapasowe** najpierw czyta, co tam jest zapisane, i to wymienia: które kopie znikną, które z nich nie istnieją nigdzie indziej oraz które repozytoria zostają, bo ochrona przed usunięciem odmawia, elementy nadal kopiują do nich prosto, inne miejsce lub sekcja trzyma tam swoje kopie albo nie dało się ich odczytać. Elementy, które straciłyby jedyną kopię, są wymienione osobno i zostają, chyba że włączysz **Skasuj też te**. Potwierdzasz wpisaniem nazwy miejsca, a usuwanie działa w tle.
- Działa to dla każdego rodzaju miejsca docelowego: takiego, z którego korzysta kilka sekcji, celu ustawionego dla jednej sekcji, repozytorium, które sam ustawiłeś, i folderu, do którego sekcja zapisuje własne kopie. Elementy kopiujące do usuniętego repozytorium znów podążają za swoją sekcją, tak samo jak ustawienie domyślne sekcji, które tam wskazywało. Folder sekcji nie może po prostu zniknąć, bo każda sekcja musi go mieć. Okno pyta o folder, do którego sekcje się przenoszą, a każda zachowuje pod nim swoją nazwę folderu.
- Usunięcie kopii w miejscu docelowym wymaga hasła logowania i zalogowania w instancji. Bez hasła miejsce można nadal usunąć, tylko nie da się stąd skasować jego kopii.

### Przykład: kopia poza siedzibą w Backblaze B2 {#example-s3}

Kontenery i autokopia mają leżeć także w Backblaze B2, żeby martwy serwer nie zabrał ich ze sobą. Te same kroki pasują do Wasabi, Hetzner Object Storage i większości innych usług S3.

1. **W B2** utwórz prywatny koszyk, na przykład `tower-backups`, w regionie blisko Ciebie. W **Application Keys** utwórz klucz z dostępem do odczytu i zapisu, ograniczony do tego koszyka, i zanotuj jego identyfikator oraz klucz aplikacji. B2 pokazuje klucz aplikacji raz. Ustaw cykl życia koszyka tak, by zachowywał tylko ostatnią wersję, bo inaczej B2 trzyma przycięte dane jako ukryte wersje i nalicza za nie opłaty.
2. **Dostawca.** W BombVault otwórz **Miejsca docelowe**, naciśnij **Dodaj miejsce docelowe** i wybierz **Backblaze B2** w **Usługa S3**.
3. **Logowanie.** Wpisz identyfikator klucza jako **ID klucza dostępu**, a klucz aplikacji jako **Tajny klucz dostępu**. Wpisz endpoint koszyka w **Endpoint**, na przykład `s3.eu-central-003.backblazeb2.com`, a BombVault odczyta z niego region. Wpisz `tower-backups` w **Bucket**, bo ten klucz otwiera tylko jeden koszyk.
4. **Testuj połączenie.** Powinno powiedzieć **Połączenie nawiązane** i wymienić foldery koszyka. Naciśnij **Nowy folder** i nazwij go `bombvault`. BombVault umieszcza w nim jeden folder na sekcję, na przykład `bombvault/containers`.
5. **Ochrona.** Okno wyjaśnia, że B2 potrafi zachowywać stare wersje dzięki blokadzie obiektów, czego BombVault nie umie sprawdzić. Zostaw **Ochrona przed usunięciem (append-only)** wyłączoną, chyba że koszyk naprawdę odmawia usuwania, żeby BombVault mógł przycinać według reguł miejsca.
6. **Dla których sekcji.** Zaznacz **Containers** i **Autokopia**. Autokopia niesie Twoje ustawienia, więc nowa instalacja może je przywrócić. Okno mówi, ile jest elementów i, gdy wiadomo, ile historii zabierze pierwsze kopiowanie.
7. Naciśnij **Zapisz miejsce docelowe**.

Pierwsze kopiowanie rusza po następnej kopii. Aby uruchomić je teraz, otwórz **Ustawienia, Miejsca docelowe** i naciśnij **Replikuj teraz** w wierszu sekcji w **Kopie poza siedzibą**. Nagłówek stanu mówi **Trwa kopiowanie poza siedzibę**. Pierwsze kopiowanie zabiera całą historię, więc 200 GB łączem 100 Mbit/s trwa około czterech i pół godziny. Kolejne wysyłają tylko to, co nowe.

Aby oszczędzić łącze, otwórz stronę miejsca i ustaw **Limit wysyłania (KiB/s)** w karcie **Kopiuj**, na przykład na `5120`, co czyta się jako około 5,0 MB/s (41,9 Mbit/s). Przy okazji sprawdź **Jak długo przechowywać** na tej samej stronie.

Aby zobaczyć, że zadziałało, spójrz na **Stan ochrony** w przeglądzie, gdzie **Poza siedzibą** zmienia się na **kopia jest aktualna**, oraz na kartę **Kopie zapasowe** kontenera, gdzie jego migawki podają teraz `tower-backups` jako drugie miejsce. Potem naciśnij ponownie **Pobierz zestaw odzyskiwania** w **Ustawienia, Przywracanie**, bo zestaw wymienia nowe miejsce i jego logowanie.

**Cloudflare R2 działa tak samo,** z tokenem obiektowym zamiast klucza. Wklej adres S3, który pokazuje panel R2, razem z koszykiem, a BombVault wypełni serwer i koszyk.

### Przykład: rest-server, który prowadzisz sam {#example-rest-server}

Rest-server w trybie append-only odmawia usuwania, więc kopia na nim przetrwa, nawet gdy ktoś przejmie Twój serwer. Wystarczy druga maszyna, u znajomego albo w garażu. Sparowany BombVault może to ustawić za Ciebie, zobacz [Serwer odbiorczy](#receiving-server). Aby uruchomić go ręcznie na dowolnym hoście Docker:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Ostatnia linia pyta o hasło i zapisuje użytkownika w pliku `.htpasswd` w folderze danych.

1. W BombVault naciśnij **Dodaj miejsce docelowe** i wybierz **rest-server** w **NAS lub serwer**.
2. Wpisz adres z użytkownikiem jako folderem, na przykład `http://192.168.2.20:8000/bombvault`, oraz **Nazwa użytkownika** i **Hasło**. Naciśnij **Testuj połączenie**.
3. W **Ochrona** włącz **Ochrona przed usunięciem (append-only)**. BombVault nigdy wtedy tam nie przycina i zostawia to drugiej stronie.
4. Zaznacz sekcje i naciśnij **Zapisz miejsce docelowe**.
5. Gdy pierwsza kopia tam jest, otwórz stronę miejsca i naciśnij **Przetestuj ochronę przed usunięciem teraz**. BombVault próbuje usunąć testowy obiekt. Karta powinna powiedzieć **Ostatnio: usunięcie odrzucone**, a **Ochrona przed usunięciem** w przeglądzie zmienia się na **sprawdzona**. Jeśli mówi **Ostatnio: usunięcie nieodrzucone**, brakuje `--append-only`.

Ponieważ nic stąd nie przycina tej kopii, ustaw **Budżet wzrostu (GB, 0 = wył.)** w oknie sekcji, otwieranym przez **Edytuj** w jej wierszu w **Kopie poza siedzibą**, żebyś się dowiedział, zanim dysk się zapełni. Przy certyfikacie samopodpisanym zobacz uwagę o `certs` wyżej.

## Własny folder sekcji {#own-folder}

Każda sekcja ma własny folder: repozytorium, do którego jej kopie są zapisywane najpierw. Stamtąd wychodzą kopie do innych miejsc docelowych. Nowa instalacja trzyma własne foldery na tym serwerze, pod `/mnt/user/bombvault`, i razem tworzą one miejsce docelowe **Lokalne**.

Własny folder może też leżeć w miejscu docelowym, które już masz, takim jak NAS, rest-server albo koszyk S3. Sekcja kopiuje wtedy tam bezpośrednio, bez lokalnej kopii pośrodku, a własny folder podąża za logowaniem, ochroną przed usunięciem, limitami przepustowości i regułami przechowywania tego miejsca. Miejsce wymienia sekcję w swojej karcie **Własne foldery**, a **Miejsca docelowe według sekcji** pokazuje je jako dom sekcji.

### Przenoszenie własnego folderu {#move-own-folder}

1. Otwórz miejsce docelowe, które ma przechowywać własny folder, i wybierz **Użyj jako własnego folderu** albo **Dodaj sekcje** w jego karcie **Własne foldery**. Aby przenieść kilka sekcji naraz, wybierz **Przenieś własne foldery** obok **Własne foldery tutaj** w karcie **Połączenie** miejsca Lokalne.
2. Wybierz sekcje w **Sekcje** i miejsce w **Przenieś do**. **Inne miejsce** przyjmuje folder na tym hoście albo ręcznie wpisany adres zdalnego repozytorium, z logowaniem spośród Twoich zapisanych danych logowania.
3. Przeczytaj **Co się stanie**. Dla każdej sekcji mówi, dokąd trafiają nowe kopie, czy kopia, która już jest w nowym miejscu, staje się własnym folderem, i które kopie są najpierw kopiowane.
4. **Skopiuj już wykonane kopie zapasowe** jest domyślnie włączone. Po wyłączeniu stare kopie zostają tam, gdzie są, i nadal można z nich przywracać.
5. Wybierz **Przełącz**. Gdy kopie trzeba najpierw skopiować, BombVault je kopiuje, a potem przełącza własny folder. Gdy już tam są, sprawdza je przed przełączeniem.

Po przeniesieniu nowe kopie trafiają do przeniesionego własnego folderu, także dla sekcji, która miała **Lokalne** wyłączone i kopiowała prosto do kopii. Kopia, która już leżała w nowym miejscu, staje się własnym folderem, a kopiowanie tam się kończy. Gdy ta kopia trzymała niektóre kopie dłużej niż reguły przechowywania sekcji, sekcja zachowuje to, co zachowuje którakolwiek z reguł, więc żadna nie przepada.

Gdy przeniesienie nie może się odbyć, okno mówi dlaczego w ramce pod **Co się stanie**. Gdy przeszkadza tylko kopiowanie starych kopii, wyłączenie **Skopiuj już wykonane kopie zapasowe** pozwala przenieść własny folder bez nich.

Sekcja zawsze potrzebuje własnego folderu. Usunięcie miejsca docelowego, które go trzyma, pyta o folder, do którego sekcja się przenosi, zobacz [Przenoszenie i usuwanie](#move-remove). Własny folder, który omijają nowe kopie, bo sekcja kopiuje prosto do kopii, jest oznaczony **(wył.)** tam, gdzie miejsce wymienia, kto z niego korzysta.

![Okno przenoszące własny folder, z wypełnionym Co się stanie dla jednej sekcji.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Okno przenoszące własny folder, z wypełnionym Co się stanie dla jednej sekcji.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Przykład: Containers przenoszą się na NAS {#example-own-folder}

Pula cache się zapełnia, a Ty masz miejsce docelowe o nazwie `nas`, udział SMB na drugiej maszynie. Sekcja Containers ma 61 kopii w swoim własnym folderze na tym serwerze i kopiuje do Backblaze B2.

1. Otwórz **Miejsca docelowe**, potem `nas` i naciśnij **Użyj jako własnego folderu**.
2. Zaznacz **Containers** w **Sekcje**.
3. Przeczytaj **Co się stanie**. Mówi, że nowe kopie Containers trafiają do folderu na `nas`, że 61 kopii z Lokalne jest tam najpierw kopiowanych, że kopiowanie do Backblaze B2 trwa nadal i że ochrona, limity przepustowości i alarm wzrostu pochodzą z `nas`.
4. Zostaw **Skopiuj już wykonane kopie zapasowe** włączone i naciśnij **Przełącz**.

BombVault kopiuje kopie w tle, a potem przełącza własny folder. Strona `nas` wymienia Containers w **Własne foldery**, element, który podąża za swoją sekcją, podaje `nas` jako swój dom, a następna kopia ląduje tam, z kopiowaniem do B2 wychodzącym z `nas`. Stary folder na tym serwerze zostaje tam, gdzie jest, i BombVault niczego tam nie usuwa.

### Ręczne wpisanie zdalnego adresu {#remote-primary-repositories}

Karta **Połączenie** miejsca Lokalne nadal przyjmuje zdalny adres dla własnego folderu sekcji: **Zmień** obok sekcji otwiera okno z przełącznikiem **Lokalne / Zdalne**. **Zdalne** przyjmuje adres restic, taki jak `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` albo `rclone:remote:bucket/path`, a **Ustawienia bezpieczeństwa zdalnego repozytorium podstawowego** dodają test połączenia, limity wysyłania i pobierania, ochronę append-only z tym samym testem manipulacji, jaki dostaje miejsce poza siedzibą, oraz alarm budżetu wzrostu.

Taki adres loguje się domyślnymi danymi logowania. Gdy leży w folderze miejsca docelowego, które masz, strona tego miejsca wymienia sekcję w **Już pod tym miejscem docelowym**, a przejęcie daje jej logowanie, klasę magazynu i ochronę przed usunięciem miejsca. Przeniesienie własnego folderu do miejsca docelowego przez **Użyj jako własnego folderu** jest prostszą drogą, bo okno sprawdza wykonane kopie, zanim cokolwiek się zmieni.

## Ustawienie dla elementu {#placement}

Karta **Harmonogram i miejsce docelowe** na karcie Ustawienia elementu ma rząd przycisków: **Lokalne** i po jednym przycisku na każde miejsce docelowe sekcji poza siedzibą. Zapalone przyciski dostają kopie elementu.

- Przy zapalonym **Lokalne** element jest zapisywany do repozytorium widocznego pod **Zapisane na** i kopiowany do każdego innego zapalonego miejsca. Zgaś miejsce, a nie dostanie od tego elementu niczego nowego. Samo Lokalne nie kopiuje nigdzie, co pasuje do danych, które już mają drugą kopię, na przykład do udziału leżącego na NAS.
- Przy zgaszonym **Lokalne** element jest zapisywany prosto do repozytorium bezpośredniego pierwszego zapalonego miejsca i stamtąd kopiowany do pozostałych zapalonych. Za pierwszym razem okno tworzy to repozytorium bezpośrednie.
- Jeden przycisk zostaje zapalony, bo kopia potrzebuje miejsca, dokąd trafi. Aby pominąć element w kopiach, użyj **Nie kopiuj**.

BombVault nigdy nie przenosi kopii między repozytoriami. Element, który podąża za sekcją, idzie razem z nią, gdy zmienia się jej ustawienie domyślne: jego następna kopia ląduje w nowym miejscu, a kopie, które ma, zostają, gdzie są. Miejsce wybrane na samym elemencie jest stałe od jego pierwszej kopii. Karta **Repozytorium** na karcie Ustawienia je podaje. Kopie można zmieniać w każdej chwili. Miejsce, które przestaje dostawać element, zachowuje kopie, które ma, i przycina je do własnego przechowywania przy następnym przebiegu poza siedzibą sekcji. Z miejsc append-only nie da się usuwać.

Miejsce, które element opuścił, zachowuje jego kopie. Na karcie **Kopie zapasowe** elementu i w **Przywracanie** stoi jako **Tylko starsze kopie zapasowe**, przywracanie nadal może z niego czytać, a przycinanie tam omija te kopie.

**Użyj ustawienia domyślnego** przywraca element z własnym wyborem do jego sekcji. Najpierw pyta, mówi, dokąd trafią wtedy nowe kopie, i wymienia, czego każde miejsce przestaje dostawać i co wysłałoby następne uruchomienie.

Pod rzędem karta mówi, dokąd element trafia i co faktycznie tam jest: ile miejsc go trzyma, kiedy każde miejsce było ostatnio widziane i czy spełniono 3-2-1. Miejsce to serwer z oryginalnymi danymi, każde miejsce docelowe poza siedzibą i każde repozytorium oznaczone **Poza lokalizacją**. BombVault sprawdza kopie i miejsca. Nie sprawdza części „dwa nośniki” z 3-2-1.

### Ustawienia domyślne dla sekcji {#placement-defaults}

Karta **Miejsca docelowe według sekcji** na stronie Miejsca docelowe rysuje ustawienie domyślne każdej sekcji jako przepływ 3-2-1. Każda linia od sekcji do miejsca docelowego jest przełącznikiem, **Lokalne** też, dla każdej sekcji, także dysku flash, zbiorów danych ZFS i autokopii. Kliknięcie linii lub kropki przełącza ją od razu, a linie przerywane są wyłączone. Tylko wyłączenie **Lokalne** sekcji pyta najpierw, w tym samym oknie co karta ustawienia domyślnego, bo sekcja kopiuje wtedy prosto do kopii. **Anuluj** zostawia wszystko tak, jak było. **Wykres / Przełączniki** pokazuje to samo jako rzędy przycisków. Przełącznik decyduje, dokąd trafiają nowe kopie. Elementy bez własnego wyboru i foldery projektów stosów Compose podążają od razu, więc ich następna kopia trafia do nowych miejsc. Kopie już zapisane zostają, gdzie są, i nadal można z nich przywracać. Flash, zbiory danych ZFS i autokopia kopiują do swojego folderu lokalnego i stamtąd kopiują poza siedzibę. Ich linię do **Lokalne** też można wyłączyć. Ich kopie trafiają wtedy prosto do miejsc poza siedzibą, a element ZFS z repliką i bez żadnego miejsca wysyła tylko replikę.

Przy wyłączonym **Lokalne** i włączonych tylko miejscach poza siedzibą sekcja kopiuje prosto do nich, a BombVault sam wybiera tam repozytorium. Kontenery, maszyny wirtualne i zestawy folderów trafiają do repozytorium bezpośredniego pierwszego włączonego miejsca, opisanego niżej, i stamtąd są kopiowane do pozostałych. Flash, zbiory danych ZFS i autokopia zapisują każdą kopię w każdym włączonym miejscu, a każde miejsce przechowuje je według własnego przechowywania.

Sekcja z wyłączonymi wszystkimi liniami mówi **Bez kopii zapasowej**. Jej elementy nie dostają kopii i liczą się jako niechronione.

Elementy, które na swojej stronie wybrały własne miejsca docelowe, je zachowują. Przepływ mówi, ile ich jest w sekcji, na przykład **2 wpisy się różnią**, i prowadzi do nich: do karty Ustawienia elementu, gdy jest jeden, do listy z miejscem kopiowania każdego, gdy jest kilka. **Użyj ustawienia domyślnego** tam przywraca element.

Nowe miejsce poza siedzibą dostaje każdy element, który nie jest ustawiony na Lokalne. Okno, które je dodaje, mówi, ile to elementów i, gdy wiadomo, ile historii.

### Repozytoria bezpośrednie {#direct-repositories}

Repozytorium bezpośrednie to miejsce, gdzie elementy kopiują prosto do miejsca poza siedzibą. BombVault umieszcza je obok folderu miejsca z dopisanym do nazwy `-direct` albo, gdy miejsce leży w korzeniu swojego koszyka, zasobu lub użytkownika rest-servera, w folderze wewnątrz niego nazwanym od sekcji, na przykład `containers-direct`. W przepływie na stronie Miejsca docelowe powstaje bez pytania. Wyłączenie Lokalne dla pojedynczego elementu, tak że domem staje się miejsce bez repozytorium bezpośredniego, otwiera okno z proponowanym miejscem, na przykład `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, i testem połączenia, który niczego nie tworzy. **Utwórz i użyj** tworzy repozytorium i kieruje na nie element. Repozytorium bezpośrednie nie ma własnych ustawień. Podąża za przełącznikiem, logowaniem, klasą magazynu, limitami, ustawieniem append-only i przechowywaniem swojego miejsca i zmienia się razem z nimi. Strona miejsca wymienia je w **Kopie trafiają prosto tutaj**, z każdą sekcją, która kopiuje do niego prosto, i liczbą elementów. To, do którego nic już nie kopiuje, mówi o tym tam i można je usunąć, a kopie w jego miejscu zostają. Gdy nowy klucz miejsca nie potrafi go otworzyć, repozytorium bezpośrednie zachowuje klucz, który ma, i zapis o tym mówi. Element na repozytorium bezpośrednim jest stamtąd kopiowany do pozostałych zapalonych miejsc, nigdy do miejsca, do którego należy repozytorium. Jego migawki niosą tag `bv:direct`, a każdy inny przebieg przechowywania je zachowuje, więc repozytorium bezpośrednie, które straciło link do swojego miejsca, nigdy nie starzeje się według reguł lokalnych. B2 jest osiągane przez swój endpoint S3, z identyfikatorem klucza i kluczem aplikacji wpisanymi jako klucze S3. Klucz ograniczony do własnego folderu miejsca nie dosięgnie sąsiedniego folderu, więc ogranicz klucz do folderu wyżej.

### Poza lokalizacją {#off-the-premises}

Miejsce docelowe lub repozytorium można oznaczyć **Poza lokalizacją**. Zdalne zaczynają oznaczone. Wyłącz to dla rest-servera w tym samym budynku. Oznaczenie tylko liczy miejsca i 3-2-1 na kartach. Nie zmienia żadnej kopii.

Replika ZFS jest kopią poza lokalizacją, ale nie kopią zapasową z historią. Zachowuje tylko migawki, które ustawisz, a stare wersje i pojedyncze pliki nadal pochodzą z kopii zapasowych. Idzie z jednego serwera ZFS prosto na drugi, przez SSH na serwer ZFS albo do punktu odbioru sparowanej instancji. Grupa parowania niesie tylko prośbę. Zobacz [Replika](zfs-datasets.md#replica).

### Po odbudowie {#after-a-rebuild}

Wybory dotyczące kopii żyją w własnych ustawieniach BombVault. Po odbudowie przez **Odkryj kopie zapasowe** albo Kreator awaryjny bez przywróconych ustawień ich nie ma, a kopiowanie wszystkiego znowu wysłałoby do B2 elementy, które pominąłeś. Kopie poza siedzibą każdej odbudowanej sekcji są więc wstrzymane. Mówi o tym przegląd, a karta **Miejsca docelowe według sekcji** oferuje **Potwierdź domyślne** z podglądem tego, co skopiuje następny przebieg, i nazw w kopiach, które nie mają elementu, a które możesz tam pominąć. Tylko potwierdzenie kończy wstrzymanie. Import pliku ustawień przywraca reguły i ustawienia domyślne, ale go nie kończy.

## Ochrona przed usunięciem (append-only) {#delete-protection-append-only}

Włącz ochronę przed usunięciem dla miejsca docelowego poza siedzibą, żeby ransomware albo przejęty host nie mógł usunąć ani nadpisać Twoich kopii. Druga strona (`restic/rest-server` uruchomiony w trybie `--append-only`) ją **wymusza**. BombVault ją tylko **weryfikuje** i nigdy nie pokazuje zieleni na podstawie samej deklaracji w konfiguracji. Folder na tym serwerze nie potrafi odmówić usunięcia, bo kto kontroluje BombVault, kontroluje też folder, więc miejsca lokalne nie oferują tego przełącznika.

Przełącznik należy do miejsca docelowego: sekcja, która tam kopiuje, bierze go z miejsca, więc włącz go na stronie miejsca w **Miejsca docelowe**. Zaplanowane **Sprawdzaj ochronę przed usunięciem** działa tylko wtedy, gdy jakaś sekcja kopiuje do miejsca z włączonym przełącznikiem. Druga strona musi być uruchomiona z `--append-only`. Z obrazem `restic/rest-server`, na TrueNAS tak samo jak na każdym innym hoście Docker, trafia to do zmiennej `OPTIONS`.

Niektóre miejsca w ogóle nie potrafią odmówić usunięcia, na przykład dysk w chmurze taki jak OneDrive. Jeśli trzymasz tam kopie celowo, włącz **Akceptuj bez ochrony przed usunięciem** na stronie miejsca. Przegląd pokazuje wtedy brak ochrony przed usunięciem sekcji, która kopiuje tylko do zaakceptowanych miejsc, jako ryzyko zaakceptowane, szary pierścień zamiast bursztynowej kropki, a kolor sekcji wynika z jej pozostałych kontroli. Nic więcej się nie zmienia: kopie tam nadal można usunąć, BombVault nadal przycina je według reguł przechowywania miejsca, a przełącznik znika, gdy ochrona przed usunięciem jest włączona. Kopia, która przechodzi w inne miejsce, traci akceptację. Włączenie ochrony przed usunięciem też ją czyści.

Najszybszą drogą do kopii append-only jest sparowany BombVault z [serwerem odbiorczym](#receiving-server): sam uruchamia dla Ciebie rest-server w trybie append-only.

!!! note "Udane usunięcie w `/locks/` jest oczekiwane"
    Append-only nie znaczy, że nic nigdy nie może zniknąć. restic musi zakładać i zwalniać własne blokady, więc `/locks/` pozostaje z założenia zapisywalne i usuwalne. Migawek i danych za nimi, czyli tego, za czym poszedłby ransomware, nie da się usunąć. Jeśli sam sondujesz drugą stronę, udane usunięcie w `/locks/` to poprawne zachowanie, a nie dziura w ochronie.

!!! warning "Repozytoria append-only nigdy nie są przycinane z tej maszyny"
    Miejsce append-only nigdy nie przycina stąd starych migawek. Przycinanie to zadanie drugiej strony. Ustaw dla niego **alarm budżetu wzrostu**, żeby dostać ostrzeżenie, zanim rozmiar repozytorium ucieknie.

## Test manipulacji {#tamper-test}

BombVault okresowo dowodzi gwarancji append-only, naprawdę próbując usunięcia w repozytorium poza siedzibą, skierowanego na nieistniejący obiekt. **Przetestuj ochronę przed usunięciem teraz** na stronie miejsca uruchamia go na żądanie, a **Sprawdzaj ochronę przed usunięciem** w **Ustawienia, Weryfikacja, Automatyczne kontrole** uruchamia go według harmonogramu.

Wyłączenie **Sprawdzaj ochronę przed usunięciem** nie ukrywa luki. Miejsce z ochroną przed usunięciem, którego nigdy nie testowano, czyta się wtedy „nie udowodniona, jej kontrola jest wyłączona”, a miejsce bez ochrony przed usunięciem czyta się „brak” w obu przypadkach.

Kopia, która jest wyłączona, nie jest testowana, a miejsce o tym mówi. Test połączenia nadal na niej działa, bo tylko czyta, więc kopię można sprawdzić, zanim się ją włączy.

- **Odrzucone** znaczy chronione.
- **Zaakceptowane** znaczy niechronione.
- Wynik **niejednoznaczny** (serwer nieosiągalny, błąd uwierzytelnienia) nigdy nie przestawia zapisanego werdyktu.

Prawdziwa zmiana z chronionego na niechroniony wywołuje jeden alert.

## Próby DR {#dr-drills}

BombVault oferuje dwa poziomy dowodu, że Twoje kopie naprawdę da się przywrócić, a nie tylko że istnieją.

- **Próby weryfikacji przywracania (lokalne).** BombVault okresowo uruchamia `restic check --read-data-subset` (ograniczone, nigdy pełne przywracanie zapełniające dysk). Częstotliwość ustawia się w **Ustawienia, Weryfikacja, Automatyczne kontrole**, a karta **Sprawdź kopie zapasowe** na tym samym kafelku pokazuje wynik.
- **Próbne przywracanie (poza siedzibą).** BombVault przywraca prawdziwy cel z repozytorium poza siedzibą do odizolowanej piaskownicy, weryfikuje go plik po pliku i bajt po bajcie, a potem sprząta. To dowodzi, że potrafisz odzyskać dane spoza siedziby, a nie tylko że repozytorium odpowiada.

Karta **Stan ochrony** w przeglądzie zbiera to dla każdej sekcji w cztery kontrole: aktualną kopię na tym serwerze, kopię poza siedzibą, ochronę przed usunięciem tam i próbne przywracanie stamtąd. Kliknięcie sekcji pokazuje szczegóły, a karta robi się zielona tylko na podstawie sprawdzonych faktów. Miejsce zaakceptowane bez ochrony przed usunięciem liczy się tam jako ryzyko zaakceptowane, a nie ostrzeżenie.

## Parowanie instancji {#pairing}

Kilka BombVault pracuje razem jako jedna grupa: pokazują nawzajem swój stan ochrony, przejmują dla siebie role i mogą przechowywać nawzajem swój klucz. Instancja dołącza do grupy za pomocą dwunastu słów.

![Strona parowania z jej trzema kartami kroków.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Strona parowania z jej trzema kartami kroków.](assets/screenshots/offsite/pairing-light.png#only-light)

Na pierwszej instancji otwórz **Ustawienia, Parowanie** i naciśnij **Wygeneruj frazę**. Trzy karty kroków pokazują, jak to przebiega. Dwanaście słów pojawia się w oknie z przyciskiem **Kopiuj**. Na każdej innej instancji otwórz to samo miejsce, naciśnij **Wpisz frazę** i wklej albo wpisz słowa, albo naciśnij **Wklej** w tym oknie. Słowo, którego nie ma na liście, jest nazywane razem ze swoją pozycją podczas pisania, a ostatnie słowo niesie sumę kontrolną, więc błędnie wpisane lub zamienione słowo jest wychwytywane, zanim cokolwiek się sparuje. Frazę generuj tylko na jednej instancji: dwie instancje, które obie ją generują, tworzą dwie osobne grupy. Jeśli przez minutę nikt się nie pojawi, strona oferuje dwa wyjścia: pokaż słowa ponownie, by wpisać je tam, albo wpisz słowa drugiej instancji i dołącz do jej grupy jednym krokiem. Parowanie działa bez hasła logowania, ale ustaw je: bez niego każdy, kto może otworzyć interfejs webowy, może przeczytać słowa i przez grupę prosić o role, które wydają hasło restic znajdujących się w niej instancji. Karta parowania o tym mówi, dopóki hasło nie jest ustawione. Z hasłem ponowne pokazanie frazy o nie pyta. **Opuść grupę** wyprowadza instancję z powrotem. Karta **Instancje** na stronie parowania wymienia członków grupy. Obraz tego, kto co robi dla kogo, jest na stronie Instancje.

Każdy, kto zna słowa, może dołączyć do grupy, więc traktuj je jak hasło.

**Jak członkowie się docierają.** Każda instancja poznaje swój własny adres w sieci z Twojej przeglądarki w chwili logowania, wymieniony w **Znane adresy** w karcie **Ta instancja** na stronie parowania razem z tymi, które instancja znalazła sama. Popraw go tam, jeśli przed nią stoi odwrotne proxy albo nietypowy port. W tej samej sieci członkowie ogłaszają ten adres multicastem i rozmawiają bezpośrednio, a tam, gdzie multicast nie przechodzi przez sieć kontenerów, jak w domyślnym moście Dockera, instancja zamiast tego przeszukuje własną podsieć w poszukiwaniu pozostałych podpisanym wywołaniem, na które może odpowiedzieć tylko członek grupy, więc parowanie kończy się w kilka sekund także bez relay. Jeśli nic się nie pojawi, **Nie możesz jej znaleźć?** przyjmuje jeden adres ręcznie, dla innej podsieci albo niestandardowego portu. Instancje w różnych sieciach idą przez relay, wybierany w jednym rzędzie czterech w karcie **Relay** na tej samej stronie. Każda forma ma swój obraz:

- **Relay projektu** (domyślny): `parleyport.halleluja.design`, z którego korzysta też KnightLoader. Nic do ustawiania.
- **Własny relay**: kontener [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) z Unraid Community Apps (także w **Ustawienia, Aplikacje**). Wpisz jego adres na każdej instancji, która ma z niego korzystać.
- **Instancja BombVault**: jedna z Twoich instancji, która jest już osiągalna z zewnątrz, z włączonym **Działaj jako relay**. Ta instancja odpowiada wtedy pod `/relay/connect` na swoim adresie, za odwrotnym proxy i certyfikatem, które już ma, i wpuszcza tylko Twoją grupę. Wpisz jej adres na każdej instancji, która ma z niej korzystać.
- **Brak relay**: członkowie znajdują się nawzajem automatycznie w tej samej sieci i nigdzie indziej.

**Co widzi Relay.** Przycisk o tej samej nazwie obok czterech form pokazuje to dla wybranej formy. Każde wywołanie między członkami jest zapieczętowane AES-256-GCM kluczem wyprowadzonym z dwunastu słów, a ten klucz nigdy nie opuszcza Twoich instancji. Relay poznaje hash grupujący połączenia, dla której instancji jest wiadomość, jak jest duża i kiedy przechodzi. Bezpośrednie wywołanie w sieci lokalnej jest pieczętowane tak samo i dodatkowo podpisane, więc nic nie zależy od certyfikatu samopodpisanego, który serwuje instancja. Wywołania, które coś wydają (odpowiedzi na role, hasło restic, loginy odbiorców, depozyty kluczy), są też podpisywane kluczem członka, który je wysyła, przypinanym przy pierwszym spotkaniu. Zobacz [Model bezpieczeństwa](configuration.md#security-model).

**Co podróżuje przez grupę.** Stan ochrony na stronie Instancje, prośba o sprawdzenie jednej sekcji teraz, prośby o role i odpowiedzi na nie, oferty pamięci oraz to, czego odbiorca lub pobierający potrzebuje po zezwoleniu na rolę: lokalizacje repozytoriów drugiej instancji i jej hasło restic. Dane kopii nigdy. Nadal idą prosto do backendów restic. APP_KEY też nie: hasło restic otwiera repozytoria tej instancji i nic więcej, ani jej zapisanych sekretów, sesji czy kodów odzyskiwania.

**Elementy sprzed parowania.** Instancje dodane tokenem floty oraz odbierane repozytoria i źródła pobierania ustawione z APP_KEY drugiej instancji zostają po aktualizacji i są oznaczone **Sparuj ponownie**. Nadal działają: przy pierwszym starcie BombVault zastępuje każdy zapisany APP_KEY wyprowadzonym z niego hasłem restic. Sparuj obie instancje, potem edytuj element i wybierz jego instancję. Do tego czasu takie elementy stoją w karcie **Bez instancji** na stronie Instancje.

Jedyne miejsce, które nadal przyjmuje APP_KEY ręcznie, to [Ze starego folderu z kopiami](#restore-from-another-bombvault-repo), na wypadek gdy drugiej instancji już nie ma i nie może odpowiedzieć w grupie.

## Instancje i role {#roles}

Strona **Instancje** (kafelek ustawień, który można przypiąć do paska bocznego) pokazuje grupę jako sieć tego, kto co robi dla kogo, kartę dla tej instancji i kartę dla każdego innego członka, telefonów z Androidem i serwerów ZFS. **Parowanie** w prawym dolnym rogu prowadzi do strony parowania. To, które części są włączone, ustawia się w **Ustawienia, Instancje, Wiele serwerów**: **Instancje**, **Odbiornik** i **Pobieranie**.

![Strona Instancje: grupa jako sieć i karta dla każdej instancji z kafelkami ról, jeden z nich Nieaktywne.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Strona Instancje: grupa jako sieć i karta dla każdej instancji z kafelkami ról, jeden z nich Nieaktywne.](assets/screenshots/offsite/instances-light.png#only-light)

Każda karta pokazuje stan ochrony członka i jego role jako kafelki. Rola to coś, co jedna instancja robi dla drugiej:

- **Odbiornik** trzyma kopie tego serwera poza siedzibą w rest-serverze w trybie append-only. Ten serwer wysyła po każdej kopii i nie może tam niczego usunąć, nawet jeśli ktoś go przejmie.
- **Pobierający** pobiera kopie tego serwera według własnego harmonogramu. Ten serwer nie musi być do tego włączony.
- **Serwer ZFS** przyjmuje repliki ZFS tego serwera. Każdy element ZFS wybiera go w **Replika** w swoich ustawieniach.
- **Przechowawca klucza** trzyma zapieczętowaną kopię zestawu odzyskiwania tego serwera. Zobacz [Przechowawca klucza](#key-keeper).

Włączenie roli wysyła prośbę, a nic nie startuje, dopóki ktoś na drugiej instancji nie naciśnie **Zezwól**. Prośba pokazuje się na karcie tej instancji jako plakietka, a jej **Szczegóły** wymieniają ją z **Zezwól** i **Odrzuć**. Dozwoloną rolę można tam w każdej chwili odwołać. Linia przerywana w sieci to rola wciąż oczekująca. Instancja na BombVault 9.x nie potrafi odpowiedzieć na prośbę, więc rola jest tam aktywna, gdy tylko ją włączysz, a prośba od instancji 9.x czeka tutaj na **Zezwól**.

**Szczegóły** na karcie otwierają resztę: odbierane repozytoria i źródła pobierania tego członka, oferty pamięci, monitorowanie, **Usuń** i **Zapomnij klucz**.

Członek może zaoferować drugiemu przez grupę własną pamięć poza siedzibą. Drugi administrator widzi ofertę w **Szczegóły** członka i ją przyjmuje albo odrzuca. Przyjęcie tworzy zapisane dane logowania i zwykłe miejsce docelowe, po tych samych kontrolach co miejsce, które wpisujesz. W ten sposób podróżują tylko dane połączenia, nigdy dane kopii. **Sprawdź teraz** prosi członka o weryfikację repozytorium jednej sekcji. Nic na stronie nie może uruchomić kopii, przywracania ani usunięcia na innej maszynie.

Wyłączone instancje i telefony z zamkniętą aplikacją zachowują swoją kartę jako **Nieaktywne**, na szaro, z informacją, kiedy ostatnio je widziano, i stoją przyciemnione w sieci, dopóki ich nie **Usuń**. Zaraz po starcie tego serwera taka karta mówi **Sprawdzanie**, dopóki grupa nie wróci. **Usuń** działa na takiej karcie niezależnie od tego, które części są włączone, i kończy też otwarte prośby członka, jego bezczynne gniazda odbioru ZFS i oczekujące prośby o klucz. Karta **Instancje** na stronie parowania wymienia tych członków także jako **Nieaktywne**.

## Odbiornik i pobierający {#receiver}

Na maszynie, która **odbiera** kopie append-only od innego BombVault, odbierane repozytoria dają niezależne monitorowanie tylko do odczytu na sprzęcie odbiorcy, żeby cicha awaria po drugiej stronie nie przeszła niezauważona. Włącz **Odbiornik** w **Ustawienia, Instancje**, potem otwórz **Szczegóły** instancji wysyłającej na stronie Instancje i dodaj odbierane repozytorium (tylko do odczytu, otwierane hasłem restic instancji wysyłającej, które dostaje przez [grupę parowania](#pairing)). Dostajesz:

- **Inwentarz według źródła**, żebyś widział dokładnie, które kontenery, maszyny wirtualne i zestawy folderów dotarły.
- **Ostatnio odebrano** dla każdego źródła, żebyś wiedział, jak świeże jest każde z nich.
- **Niezależny `restic check`** uruchamiany na sprzęcie odbiorcy, żeby integralność była weryfikowana tam, gdzie dane naprawdę leżą, a nie tylko u nadawcy.
- **Wyłącznik martwego człowieka:** alert, gdy źródło przestaje wysyłać w ustawionym oknie.
- **Alerty integralności:** alert, gdy kontrola po stronie odbiorcy się nie powiedzie.

Odbiornik jest ściśle tylko do odczytu. Nigdy nie zapisuje do odbieranego repozytorium, więc nigdy nie może złamać gwarancji append-only, na której polega nadawca.

Pobierający działa w drugą stronę: z włączonym **Pobieranie** instancja czyta kopie innego członka według własnego harmonogramu, do osobnego repozytorium dla każdego źródła, więc nigdy nie liczą się jako własne kopie tej maszyny. Jego źródła stoją także w szczegółach tego członka: lokalizacja repozytorium, rodzaj kopii, jaką zawiera, i jak często pobierać. Hasło restic przychodzi przez grupę, nigdy APP_KEY, a druga strona nie ma nic więcej do ustawienia i nie musi działać. Źródło jest tylko czytane, a każda strona zachowuje własne dane logowania. Źródło `rclone:` jest odrzucane, bo rclone dotarłoby do niego zasobami tej instancji.

### Serwer odbiorczy {#receiving-server}

Maszyna odbierająca może też uruchamiać rest-server, do którego kopiują pozostałe. Ustawia się go przy pierwszym zezwoleniu na prośbę **Odbiornik**, która o to prosi: okno mówi, że serwer potrzebuje serwera odbiorczego, i pyta o folder na udziale, z **Nowy folder** do utworzenia jednego, oraz o port (8000, chyba że używa go inny kontener). BombVault następnie:

1. odmawia, jeśli kontener o nazwie `rest-server` już istnieje albo port zajmuje inny kontener,
2. pobiera `restic/rest-server` i uruchamia go przez gniazdo Docker w trybie append-only, z prywatnymi repozytoriami i plikiem logowania w folderze,
3. zapisuje jego szablon Unraid na dysku flash, żeby kontener pozostał edytowalny w zakładce Docker, albo oferuje szablon do pobrania, gdy dysk flash jest poza zasięgiem,
4. uruchamia na nim test manipulacji i pokazuje, czy odmawia usuwania.

Instancje Twojej grupy znajdują potem serwer wśród dostawców w **Z twojej grupy**, nazwany od maszyny odbierającej, a rola **Odbiornik** oferuje go jako bezpieczny wybór. Każda instancja dostaje własny login przy pierwszym użyciu serwera i zapisuje tylko do własnego folderu tam. Przy włączonym **Odbiornik** w **Ustawienia, Instancje**, **Serwer odbiorczy** na karcie tej instancji otwiera serwer ponownie. Jego okno wymienia loginy w **Partnerzy z loginem**, a **Odwołaj login** zabiera jeden. To, co ta instancja już skopiowała, zostaje w folderze. Konfiguracja tworzy też jeden **Login spoza grupy**, którego hasło okno pokazuje raz.

Instancja, która dociera do maszyny odbierającej tylko przez relay, nie może używać serwera, bo relay nie niesie kopii. Najpierw dodaj adres maszyny odbierającej na stronie parowania. Gdy BombVault działa na własnym adresie IP (na przykład na br0), wypełnij **Adres dla partnerów**, bo serwer nasłuchuje na adresie hosta.

## Przykład: parowanie dwóch maszyn Unraid {#worked-example-two-unraid-boxes-end-to-end}

To jedno kompletne ustawienie opisanych wyżej części, bo łatwiej je złożyć, gdy zobaczysz je złożone.

Dwie maszyny: **TOWER** uruchamia kontenery i wysyła kopie. **VAULT** je odbiera i wymusza append-only. Podstaw własne nazwy, adresy i ścieżki udziałów.

**1. Sparuj obie maszyny.** Na TOWER otwórz **Ustawienia, Parowanie**, naciśnij **Wygeneruj frazę** i wpisz dwanaście słów na VAULT w **Wpisz frazę**. Najpierw ustaw hasło logowania na obu.

**2. Na TOWER poproś VAULT, by został odbiornikiem.** Na stronie Instancje włącz **Odbiornik** na karcie VAULT, wybierz sekcje i **serwer odbiorczy VAULT** jako miejsce, a potem wyślij prośbę.

**3. Na VAULT zezwól.** Strona Instancje na VAULT pokazuje prośbę na karcie TOWER. Naciśnij **Zezwól**. Za pierwszym razem VAULT pyta o folder dla swojego serwera odbiorczego, na przykład `user/appdata/rest-server`, uruchamia rest-server w trybie append-only i przeprowadza na nim test manipulacji.

**4. Z powrotem na TOWER.** Od tej chwili TOWER kopiuje na VAULT po każdej kopii, a nowe miejsce docelowe pojawia się na stronie Miejsca docelowe TOWER z jego ochroną przed usunięciem.

Co odpowiada test manipulacji:

| Wynik | Co się stało |
| --- | --- |
| **chroniony** | VAULT odrzucił usunięcie. To jedyny stan zaliczony. |
| **niechroniony** | VAULT przyjął usunięcie. `--append-only` brakuje albo go usunięto. |
| **niejednoznaczny** | Ani jedno, ani drugie. Zwykle adres nie jest tym, którego używa sam restic, albo zmieniły się dane logowania. Nic nie jest zapisywane i żaden alert nie wychodzi. |

**5. Obserwuj, co dociera, jeśli chcesz.** Włącz **Odbiornik** w **Ustawienia, Instancje** na VAULT, otwórz **Szczegóły** TOWER i dodaj odbierane repozytorium. VAULT dostaje hasło restic TOWER przez grupę. Nikt nie wpisuje klucza.

**6. Zrób to wzajemnie.** Powtórz kroki w drugą stronę: VAULT prosi TOWER, by został jego odbiornikiem. Każda maszyna wymusza wtedy append-only dla drugiej i żadna nie może usunąć kopii drugiej.

Rest-server prowadzony ręcznie też działa: dodaj go przez **Dodaj miejsce docelowe** jako rest-server, z użytkownikiem i hasłem z jego `.htpasswd`, i włącz ochronę przed usunięciem. Zachowaj `--append-only` w jego opcjach. Bez tego druga strona jest znowu zwykłym udziałem.

!!! warning "Ścieżka lokalna to ścieżka wewnątrz kontenera"
    Tam, gdzie BombVault prosi o folder na tym serwerze, wpisz go względem montowania hosta, na przykład `user/appdata/rest-server/bombvault-containers`, a nie `/mnt/user/appdata/...`. BombVault działa w kontenerze, gdzie `/mnt` hosta jest zamontowane gdzie indziej. Bezwzględna ścieżka hosta tam nie istnieje. Jeśli wkleisz taką, BombVault poda względną ścieżkę, której należy użyć.

## Kreator awaryjny {#emergency}

**Kreator awaryjny** przywraca serwer, którego już nie ma albo który postawiono od nowa. Zaczyna się na stronie **Przywracanie** w **Awaria** oraz od **Przywróć z kopii zapasowej** na stronie powitalnej nowej instalacji. Przebiega w czterech krokach: **Dostęp**, **Ustawienia**, **Wybór** i **Przywracanie**.

1. **Dostęp.** Pokaż BombVault klucz starych kopii na jeden z trzech sposobów:
    - **Wgraj zestaw odzyskiwania**: plik pobrany podczas konfiguracji. Zawiera klucz i każde miejsce docelowe, a BombVault resztę uzupełnia z niego. Zestaw zapieczętowany przez age prosi o tożsamość, dla której go zapieczętowano. Jest używany raz i nie jest zapisywany.
    - **Przez grupę**: instancja Twojej grupy trzyma klucz jako depozyt. Potrzebujesz dwunastu słów grupy, swojego kodu odblokowania i kogoś przy tej instancji. Zobacz [Przechowawca klucza](#key-keeper).
    - **Wpisz klucz**: `APP_KEY` starego serwera i miejsce, gdzie leżą jego kopie.

    BombVault próbuje potem klucza w każdym miejscu docelowym i dla każdego mówi, czy jest czytelne, nie znaleziono, jest zablokowane innym kluczem, jest nieosiągalne albo nadal brakuje mu danych dostępowych. Nic nie zostało jeszcze zapisane, a klucz zostaje w pamięci na ograniczony czas. **Przejmij klucz** sprawia, że ten serwer od teraz zapisuje swoje kopie tym kluczem. **Cofnij przejęcie** wraca do jego własnego klucza, dopóki nie wykona się pierwsza kopia. Serwer, który ma już własne kopie, odmawia przejęcia i wskazuje zamiast tego [Ze starego folderu z kopiami](#restore-from-another-bombvault-repo). **Zapisz dane dostępowe** zachowuje dane logowania, które zestaw niesie dla zdalnych miejsc docelowych, i pyta, zanim zastąpi dane logowania, które ten serwer już ma.
2. **Ustawienia.** **Przywróć ustawienia** przywraca własne ustawienia BombVault z najnowszej autokopii: miejsca docelowe, dane logowania, harmonogramy i reguły. BombVault restartuje się, żeby je zastosować, przez gniazdo Docker, więc działająca baza ustawień nigdy nie jest nadpisywana pod otwartym uchwytem, a kreator potem sam idzie dalej. Logowanie tego serwera zostaje: nadal logujesz się hasłem, którego używasz teraz. Sekrety, które miał stary serwer i których nie da się przejąć, są wymienione, żebyś mógł wpisać je ponownie. Przywrócenie kończy każdą sesję i usuwa klucze dostępu, więc ustaw je potem od nowa.
3. **Wybór.** BombVault przegląda Twoje kopie i wymienia znalezione kontenery, maszyny wirtualne, zestawy folderów i zbiory danych ZFS. Najnowsza migawka jest wszędzie zaznaczona z góry. To, co ten serwer nadal ma, zaczyna wyłączone, bo przywracanie by to zastąpiło. Zestawy folderów po odbudowie nie wiedzą już, skąd pochodzą, więc trafiają do wybranego przez Ciebie folderu.
4. **Przywracanie.** Wszystko zaznaczone wraca naraz, zostawione zatrzymane, jeśli chcesz sam uruchamiać każdy kontener i maszynę, co jest najbezpieczniejsze po całkowitej utracie. Potem pobierz nowy zestaw odzyskiwania: stary należy do poprzedniej instalacji.

![Kreator awaryjny w pierwszym kroku, z trzema drogami wejścia.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Kreator awaryjny w pierwszym kroku, z trzema drogami wejścia.](assets/screenshots/offsite/emergency-light.png#only-light)

### Przykład: nowy serwer z zestawu odzyskiwania {#example-emergency}

Serwer TOWER zginął razem z dyskami. Miał 23 kontenery i 2 maszyny wirtualne, a kopiował je i swoją autokopię do Backblaze B2. Masz `bombvault-recovery-kit.md` z menedżera haseł i nową maszynę z Unraid. Jej dawny `APP_KEY` przepadł i nie jest Ci potrzebny.

1. **Zainstaluj BombVault** na nowej maszynie i nadaj mu nowy `APP_KEY`, jak przy każdej nowej instalacji. Ustaw połączenie dla maszyn wirtualnych w **Ustawienia, Integracje, SSH hosta**, jeśli chcesz je odzyskać, bo przywracanie maszyny go wymaga.
2. **Uruchom kreator.** Na **Witamy w BombVault** wybierz **Przywróć z kopii zapasowej**. Na stronie Przywracanie wybierz **Uruchom kreator awaryjny** w **Awaria**.
3. **Dostęp.** Wybierz **Wgraj zestaw odzyskiwania** i upuść plik. BombVault próbuje jego klucza w każdym miejscu docelowym i mówi na przykład **Zestaw odzyskiwania odczytany. Jego klucz otwiera 0 z 2 miejsc docelowych.** Folder lokalny mówi **nie znaleziono**, bo dysk zginął, a Backblaze B2 mówi **brak danych dostępowych**.
4. **Zapisz dane dostępowe.** Zestaw niesie dane logowania swoich zdalnych miejsc. Naciśnij **Zapisz dane dostępowe**, a miejsca zostaną sprawdzone ponownie, więc Backblaze B2 mówi teraz **czytelne**.
5. **Przejmij klucz.** Ten serwer zapisuje teraz swoje kopie starym kluczem, więc nowe kopie pasują do starych. Zachowuje własny `APP_KEY` w szablonie. **Cofnij przejęcie** działa, dopóki nie wykona się pierwsza kopia.
6. **Ustawienia.** Wybierz **Przywróć ustawienia**, co czyta najnowszą autokopię z B2. BombVault restartuje się, żeby ją zastosować, a kreator sam idzie dalej. Twoje miejsca docelowe, harmonogramy i reguły wracają, a Ty nadal logujesz się hasłem tego serwera. Wpisz ponownie sekrety, które wymienia kreator, na przykład ustawienia powiadomień.
7. **Wybór.** Ustaw **Gdzie są kopie zapasowe?** na kopie poza siedzibą, bo lokalne zginęły ze starym serwerem. BombVault wymienia 23 kontenery i 2 maszyny wirtualne z najnowszą migawką zaznaczoną z góry, a dla każdej możesz wybrać inną.
8. **Przywracanie.** Włącz **Zostaw zatrzymane po przywróceniu**, co jest najbezpieczniejsze po całkowitej utracie, i uruchom. **Co się stanie** mówi, co wróci. Linia wyniku brzmi na przykład **Przywrócono 25, 0 niepowodzeń. Uruchom je w Kopie zapasowe, gdy będzie trzeba.**
9. **Zakończ.** Najpierw uruchom bazy danych, potem aplikacje, które z nich korzystają. Otwórz **Ustawienia, Przywracanie** i pobierz nowy zestaw odzyskiwania, bo stary należy do poprzedniej instalacji. Sprawdź **Wymaga uwagi** w przeglądzie i raz uruchom **Utwórz kopię wszystkiego teraz**, gdy wszystko działa.

Jeśli brakuje też autokopii, wybierz **Pomiń, nie mam autokopii**. Miejsca docelowe pochodzą wtedy tylko z zestawu, harmonogramy i reguły ustawiasz od nowa, a kopie poza siedzibą są wstrzymane, dopóki nie potwierdzisz ustawień domyślnych sekcji, zobacz [Po odbudowie](#after-a-rebuild).

!!! tip "Planowana migracja a awaria"
    Kreator awaryjny przywraca własne ustawienia BombVault z kopii zapasowej. Przy *planowanej* przeprowadzce na nową maszynę możesz zamiast tego przenieść konfigurację bezpośrednio za pomocą karty **Eksport / import ustawień** (przenośny plik JSON). Zobacz [Konfiguracja](configuration.md#portable-settings-export-and-import).

### Ze starego folderu z kopiami {#restore-from-another-bombvault-repo}

**Ze starego folderu z kopiami** w **Inne źródła** na stronie Przywracanie otwiera repozytorium *innej* instancji BombVault (udział zamontowany pod `/mnt` albo zdalny adres) z zestawem odzyskiwania tej instancji lub wpisanym jej `APP_KEY`, w jednorazowej sesji tylko do odczytu. Przeglądaj zapisane tam kontenery, maszyny wirtualne, zestawy folderów i zbiory danych ZFS, wybierz migawkę i przywróć ją, a przywrócony obiekt staje się zwykłym lokalnym kontenerem, maszyną wirtualną lub zestawem folderów. Do drugiego repozytorium nigdy nic nie jest zapisywane, a Twoje własne ustawienia kopii pozostają nietknięte (sesja żyje w pamięci i wygasa sama). Przeniesienie kontenera z serwera A na serwer B nie oznacza przestawiania ustawień i cofania ich potem. To okno jest jednorazowe: otwiera sesję, przywraca to, co wybierzesz, i zapomina drugą instancję. Jeśli zamiast tego chcesz stałego układu, w którym ta maszyna według harmonogramu pobiera migawki innej instancji do własnego repozytorium, to rola **Pobierający** na stronie Instancje.

Kontener, którego sieć nie istnieje na tym serwerze, jak sieć `br0` z Unraid na zwykłym hoście Docker, pokazuje pod swoim wierszem wybór sieci. BombVault tworzy go w wybranej sieci, razem z jego pozostałymi sieciami. Stały IP i adres MAC należały do starej sieci i są odrzucane, więc nowa sieć przydziela je sama.

## Przechowawca klucza {#key-keeper}

Przechowawca klucza to instancja Twojej grupy, która trzyma zapieczętowaną kopię zestawu odzyskiwania tego serwera. Sama nie potrafi jej otworzyć. Jeśli ten serwer zginie, nowa instalacja, która dołącza do Twojej grupy, prosi przechowawcę o kopię i otwiera ją Twoim kodem odblokowania.

![Okno kodu odblokowania: dwadzieścia znaków w czterech grupach, pokazane raz.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Okno kodu odblokowania: dwadzieścia znaków w czterech grupach, pokazane raz.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Na stronie Instancje włącz **Przechowawca klucza** na karcie instancji, która ma go trzymać, i wybierz **Złóż depozyt**. Złożenie depozytu wymaga hasła logowania.
2. Ten serwer pokazuje **Kod odblokowania** raz: dwadzieścia liter i cyfr w czterech grupach po pięć. Zapisz go i potwierdź przez **Mam zapisany kod**. Ten serwer nie przechowuje kodu, tylko to, czego potrzebuje, by zapieczętować dla niego depozyt.
3. Przechowawca trzyma teraz depozyt, a ten serwer odnawia go, gdy zestaw się zmienia. **Depozyt klucza** na karcie tej instancji wymienia **Depozyt tego serwera** z każdym przechowawcą i jego stanem oraz **Depozyty przechowywane na tym serwerze** dla innych. **Utwórz nowy kod** zastępuje kod. Każdy przechowawca przełącza się na nowy, gdy dostaje nowy depozyt.

W tym dniu potrzebujesz trzech rzeczy: dwunastu słów swojej grupy, kodu odblokowania i dostępu do interfejsu przechowawcy, gdzie ktoś musi nacisnąć **Zezwól**. W Kreatorze awaryjnym wybierz **Przez grupę**, wskaż depozyt, poproś o niego i wpisz kod odblokowania. Kod jest używany na nowym serwerze i nie jest wysyłany do przechowawcy. U przechowawcy prośba pokazuje się w **Depozyt klucza** z informacją, kto prosi, czy ta instancja była tam kiedykolwiek widziana i jak często prosiła. **Zezwól** wydaje zapieczętowaną kopię, a zezwolenie wymaga hasła logowania. Nowa instalacja ma 30 minut na odebranie jej po zezwoleniu, prośba, na którą nikt nie odpowie, wygasa po dobie, a instancja, która prosi zbyt często, musi poczekać godzinę.

Bez kodu nikt nie otworzy kopii, przechowawca też nie. Jeśli kod zgubisz, utwórz nowy, gdy ten serwer jeszcze działa. Jeśli serwera już nie ma, zestaw odzyskiwania i wpisany APP_KEY nadal działają. Przechowawca może w każdej chwili usunąć depozyt, a właściciel musi wtedy poprosić ponownie.

## Zestaw odzyskiwania {#recovery-kit}

To element, który umożliwia odzyskiwanie po awarii nawet wtedy, gdy nie działa żaden BombVault.

**Pobierz zestaw odzyskiwania** w karcie **Na wypadek awarii** w **Ustawienia, Przywracanie** daje **klucz**, **wyprowadzone hasło restic**, każde **miejsce docelowe** z repozytorium każdej sekcji, dane logowania, których nowa instalacja potrzebuje, by do nich sięgnąć, oraz **dokładne polecenia restic**, żebyś mógł przywracać prosto przez CLI restic na dowolnej maszynie. Wymaga hasła logowania. Zestaw drukuje klucz także przy wyłączonym szyfrowaniu, bo definicje kontenerów i maszyn wirtualnych są zawsze nim pieczętowane. Po przejęciu drukuje przejęty klucz kopii i APP_KEY tego serwera, którego nowa instalacja potrzebuje tylko do przywrócenia ustawień. Karta **Zapisz swój zestaw odzyskiwania** w przeglądzie przypomina o tym, dopóki go nie zapiszesz.

!!! danger "Przechowuj zestaw odzyskiwania poza serwerem"
    Zestaw zawiera sekret, który odszyfrowuje Twoje kopie. Trzymaj go w bezpiecznym miejscu, osobno od serwera (menedżer haseł, wydruk w sejfie). Jeśli stracisz zarówno BombVault, jak i `APP_KEY`, bez zestawu odzyskiwania i bez depozytu klucza, Twoich zaszyfrowanych kopii nie da się odzyskać.

!!! warning "Najnowsza migawka nie zawsze jest tą do przywrócenia"
    Od restic 0.17 `restic snapshots` pokazuje rozmiar każdej migawki. Po utracie danych najnowsza migawka może być opróżniona, więc nie przywracaj migawki, która jest znacznie mniejsza od poprzednich. Po ransomware może to być zaszyfrowana migawka o zwykłym rozmiarze. Jeśli BombVault nadal działa, najpierw sprawdź jego stronę **Anomalie**: wskazuje ostatnią dobrą kopię, a okno przywracania zaznacza ją z góry. Przywracanie nie potrzebuje żadnych danych anomalii BombVault, a wstrzymanie przechowywania zawsze tylko zachowuje więcej migawek.

### Pieczętowanie zestawu {#sealing-the-kit}

Jeśli włączyłeś **Szyfruj eksporty za pomocą age** w **Ustawienia, System**, zestaw jest nim także pieczętowany i pobiera się jako `bombvault-recovery-kit.md.age`. Jest w pancerzu ASCII, a nie binarny, więc nadal jest zwykłym tekstem: wklejenie do menedżera haseł albo wydruk działa dokładnie jak przedtem, a zawartość jest nieczytelna bez Twojego klucza.

!!! warning "Nie przechowuj klucza age wewnątrz zestawu"
    Do otwarcia zapieczętowanego zestawu potrzebujesz swojego **prywatnego** klucza age. Trzymaj go w miejscu, które nie zależy od samego zestawu, inaczej będziesz mieć dwie rzeczy do odzyskania zamiast jednej. Pieczętowanie się opłaca, gdy zestaw leży w miejscu, którego nie w pełni kontrolujesz (współdzielony menedżer haseł, notatki w chmurze, wydruk w biurze). Zestaw we własnym sejfie jest już chroniony przez sejf.

    Przy włączonym szyfrowaniu i bez użytecznego odbiorcy pobranie jest odrzucane od razu. BombVault nigdy nie wraca do wydawania klucza głównego jawnym tekstem.

### Jeśli nie masz zestawu pod ręką {#if-you-do-not-have-the-kit-to-hand}

Hasło nie jest nigdzie zapisane, jest **obliczane** z klucza kopii, więc możesz je odtworzyć sam, mając tylko klucz i powłokę:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

To HMAC-SHA256 ze stałego ciągu `bombvault:restic-repo`, z kluczem będącym surowymi bajtami szesnastkowego `APP_KEY`, wypisany jako 64 małe znaki szesnastkowe. Ta sama wartość jest w zestawie, podana jako wyprowadzone hasło restic. To na dzień, gdy zestaw leży tam, gdzie Cię nie ma. Na serwerze, który przejął kopie innego, użyj przejętego klucza kopii, czyli dawnego APP_KEY tamtego serwera.

!!! warning "Dla odebranego repozytorium użyj klucza instancji WYSYŁAJĄCEJ"
    Repozytorium, które trafiło tu przez kopię poza siedzibą, utworzyła maszyna, która je wysłała, ze **swoim** kluczem. Wyprowadzenie z klucza maszyny odbierającej daje hasło, które restic odrzuci, co wygląda dokładnie jak uszkodzone repozytorium, a nim nie jest. To zwykły powód, dla którego `restic check` na odebranym repozytorium raz po raz prosi o hasło.

Ponieważ definicje odzyskiwania leżą **wewnątrz** każdego repozytorium (`<repo>/def`, `<repo>/vm-def`), skopiowany folder repozytorium jest w pełni samodzielny, więc zestaw plus repozytorium to wszystko, czego potrzebuje przywracanie na pustym sprzęcie.

## Odzyskiwanie zrzutu bazy danych {#database-dumps}

Zrzut bazy danych to osobny punkt przywracania w repozytorium kontenerów, oznaczony tagiem `dbdump:<container>`, zawierający jeden plik `/dbdump/<container>.sql`. BombVault wymienia, pobiera i importuje je na karcie **Kopie zapasowe** kontenera. Poniżej te same kroki samym restic, na dzień, gdy BombVault nie ma.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Tagi `dbversion:` i `dbname:` na każdym zrzucie mówią, z jakiej wersji serwera pochodzi i jakie bazy zawiera. Kompletny plik kończy się `-- PostgreSQL database cluster dump complete` albo `-- Dump completed`.

Zaimportuj go do kontenera tej samej lub nowszej wersji (PostgreSQL) albo tej samej wersji głównej (MySQL i MariaDB), uruchomionego raz z pustym folderem danych, żeby się zainicjował. Host nie potrzebuje klienta bazy danych, kontener go ma:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Dla pojedynczej bazy z pełnego zrzutu MySQL i MariaDB przyjmują `--one-database <name>` w poleceniu klienta. Zrzut PostgreSQL ma jedną sekcję na bazę, każdą zaczynającą się linią `\connect <name>`: skopiuj tę sekcję do osobnego pliku i zaimportuj z `-d <name>` po utworzeniu bazy.

!!! warning "Zrzut roota niesie użytkowników serwera"
    Pełny zrzut MySQL lub MariaDB wykonany jako root zawiera systemową bazę `mysql`, więc import zastępuje konta nowego serwera, razem z hasłem roota, tymi ze zrzutu. W PostgreSQL `role ... already exists` dla użytkownika, którego utworzył kontener, jest oczekiwane i niegroźne.
