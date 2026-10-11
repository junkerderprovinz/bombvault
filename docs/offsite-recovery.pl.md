# Kopie poza siedzibą i odzyskiwanie

!!! note "Kopie poza siedzibą czekają po odbudowie"
    Gdy Kreator awaryjny lub **Odkryj kopie zapasowe** odbudowuje elementy bez dawnych ustawień, kopie poza siedzibą tych sekcji są wstrzymane, dopóki ich ustawienie domyślne nie zostanie potwierdzone. Zobacz [Po odbudowie](#after-a-rebuild).

Lokalne kopie zapasowe chronią przed utraconym kontenerem lub złą aktualizacją. Kopie poza siedzibą i przetestowany zestaw odzyskiwania chronią przed utratą całej maszyny, ransomware albo pożarem. Ta strona omawia miejsca docelowe i kopie poza siedzibą, zabezpieczanie kopii przed manipulacją, dowodzenie, że potrafisz przywrócić dane, parowanie kilku serwerów i odzyskanie wszystkiego, gdy samego BombVault już nie ma.

## Kopie poza siedzibą {#off-site-copies}

Zachowaj szybką kopię lokalną i dodaj jedno lub więcej miejsc docelowych poza domem. BombVault kopiuje tam nowe migawki poleceniem `restic copy` w trybie best-effort, więc potknięcie poza siedzibą nigdy nie powoduje niepowodzenia kopii lokalnej. W tym układzie repozytorium lokalne pozostaje podstawowe, a repozytorium poza siedzibą jest kopią, ale repozytorium podstawowe sekcji wcale nie musi być lokalne; zobacz [Zdalne repozytoria podstawowe](#remote-primary-repositories) poniżej, jeśli zamiast kopiować chcesz tworzyć kopię prosto do S3, rest-servera i podobnych.

- **Kilka miejsc docelowych na sekcję.** Każda sekcja (kontenery, maszyny wirtualne, flash, autokopia, foldery i zbiory danych ZFS) może kopiować naraz do kilku miejsc docelowych poza siedzibą, więc możesz trzymać na przykład rest-server u znajomego i koszyk S3 równolegle. Karta **Miejsca docelowe według sekcji** na stronie Miejsca docelowe włącza lub wyłącza każdą linię od sekcji do miejsca docelowego.
- **Kiedy działają kopie.** Karta **Kopiuj** miejsca docelowego ustawia **Kiedy kopiować**: **Po każdej kopii zapasowej** albo według własnego planu, który zbiera migawki i kopiuje w ustalonych porach. W **Ustawienia, Harmonogram, Kopie poza siedzibą według obszarów** **Wyjątek** daje jednej sekcji własny plan kopiowania. **Replikuj teraz** w karcie **Kopia off-site** sekcji w **Ustawienia, Miejsca docelowe** uruchamia kopiowanie na żądanie.
- **Przechowywanie** ustawia się osobno dla każdego miejsca docelowego w **Jak długo przechowywać**, więc kopie poza siedzibą możesz trzymać dłużej jako archiwum. Kopia w polu sekcji poza siedzibą podąża za kartą **Przechowywanie poza siedzibą** w **Ustawienia, Miejsca docelowe**; zostaw same zera, aby nigdy nie przycinać migawek poza siedzibą automatycznie.
- **Limity przepustowości** (**Przepustowość poza siedzibą** w **Ustawienia, Miejsca docelowe** albo karta **Kopiuj** miejsca docelowego) ograniczają tempo wysyłania/pobierania restic, żeby kopie nie nasycały Twojego WAN.
- Gdy kopia trwa, nagłówek stanu przeglądu pokazuje **Trwa kopiowanie poza siedzibę**. To aktywny wskaźnik, a nie pasek procentowy, bo `restic copy` nie udostępnia postępu w formie czytelnej dla maszyn.

!!! note "Przywracanie z dowolnego miejsca"
    Każdy element pokazuje swoje kopie jako jedną oś czasu we wszystkich miejscach, gdzie leży kopia, na karcie **Kopie zapasowe**. Kopia skopiowana do B2 pojawia się raz, oznaczona każdym miejscem, które ją przechowuje. Przywracanie bierze pierwsze miejsce, do którego sięga, zaczynając od repozytorium, do którego element jest zapisywany, a dla każdej migawki możesz wskazać inne. Miejsca docelowe poza siedzibą są odczytywane dopiero po kliknięciu **Sprawdź**, bo listowanie pamięci w chmurze trwa. Usuwanie w jednym miejscu najpierw sprawdza pozostałe i mówi, czy to była ostatnia kopia.

## Miejsca docelowe {#destinations}

Miejsce docelowe to miejsce, do którego trafiają kopie zapasowe: folder lokalny, udział, własny serwer albo usługa w chmurze. Ustawia się je raz i obsługuje potem każdą sekcję. Otwórz **Miejsca docelowe** z kafelka ustawień albo włącz tam **Pokaż także na pasku bocznym**, aby je przypiąć.

**Dodaj miejsce docelowe** otwiera okno w czterech krokach:

1. **Dostawca.** Każda usługa ma swoje logo, nad nimi jest wyszukiwarka, a usługi są pogrupowane: usługi pamięci z koszykami S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 i inne), własny serwer S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), własny serwer i udziały (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, zamontowana ścieżka), pamięć w chmurze (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud i reszta obsługiwana przez rclone) oraz serwery odbiorcze Twojej grupy. Każda mówi, jak dobrze nadaje się do kopii zapasowych: dyski w chmurze zwalniają przy wielu żądaniach, więc pierwsza kopia i przycinanie trwają tam dłużej.
2. **Logowanie.** Pola zależą od usługi: klucz dostępu dla S3, użytkownik i hasło dla WebDAV i SMB, hasło aplikacji tam, gdzie logowanie dwuskładnikowe blokuje zwykłe, publiczny klucz SSH BombVault dla SFTP i Storage Box albo token dla usług, które logują się przez przeglądarkę. Dla nich okno pokazuje polecenie `rclone authorize` do uruchomienia na komputerze z przeglądarką; wypisany przez nie token trafia do pola. **Testuj** sprawdza logowanie, zanim cokolwiek zostanie zapisane. Okno wyświetla potem foldery w miejscu docelowym, z **Nowy folder** do utworzenia jednego i wolnym miejscem tam, gdzie usługa je zgłasza. Pusty folder jest najbezpieczniejszy; BombVault umieszcza w nim jeden folder na sekcję.
3. **Ochrona.** Okno mówi wprost, co usługa potrafi. Rest-server w trybie append-only odmawia usuwania, a test manipulacji to sprawdza. Koszyk S3 może zachowywać stare wersje dzięki wersjonowaniu i blokadzie obiektów, czego BombVault jeszcze nie umie sprawdzić. Dysk w chmurze nie potrafi odmówić usunięcia wcale: kto wejdzie na serwer, wejdzie też do tej kopii. Włącz **Ochrona przed usunięciem (append-only)** tylko tam, gdzie druga strona naprawdę odmawia usuwania; BombVault nigdy wtedy tam nie przycina. **Na wypadek awarii** mówi, co zestaw odzyskiwania zawiera dla tego miejsca.
4. **Dla których sekcji.** Wybierz sekcje, które tworzą tu kopie. Pierwsze kopiowanie zabiera całą historię. Elementy, które wybrały własne miejsca docelowe, nie dostają nowego same z siebie, a okno je wymienia.

Usługi S3 działają przez własny backend S3 restic, dzięki czemu mogą obowiązywać klasa pamięci i blokada obiektów. Każda inna usługa działa przez rclone dołączone do BombVault, a jej zdalny zasób pojawia się potem w karcie **Off-site (rclone)** w **Ustawienia, Miejsca docelowe**. Eksport ustawień zawiera miejsca docelowe; z dołączonymi danymi logowania zawiera także ich logowanie.

Każde miejsce docelowe ma własną stronę:

- **Połączenie** z nazwą, przełącznikiem **Dostępne** (wyłączone znaczy, że BombVault już tam nie zapisuje, a elementy tworzące tam kopie kończą się czytelnym komunikatem zamiast trafiać gdzie indziej), folderem, kompresją, klasą pamięci i **Poza lokalizacją**. **Zmień połączenie** testuje i zapisuje nowy adres lub logowanie.
- **Dane logowania**: zestaw współdzielony albo własny.
- **Ochrona przed usunięciem**, z **Przetestuj ochronę przed usunięciem teraz** i informacją, kto przycina tę kopię.
- **Kopiuj**: kiedy kopiować oraz limity wysyłania i pobierania.
- **Jak długo przechowywać**: **Krótko**, **Zrównoważone**, **Długo** albo **Własne wartości**, z osią czasu pokazującą, które kopie reguła zachowuje. **Pytaj przed zachowaniem mniej** pyta, zanim krótsza reguła usunie kopie, których jedyny egzemplarz leży tutaj.
- **Reguły według sekcji**: własne repozytorium, przechowywanie lub kompresja jednej sekcji w tym miejscu docelowym. **Zmień reguły** je otwiera, a **Przywróć ustawienia miejsca docelowego** przywraca sekcję.
- **Już pod tym miejscem docelowym**: sekcje, których wpisany ręcznie adres leży w folderze tego miejsca. Przejęcie jednej zostawia repozytorium tam, gdzie jest; logowanie, klasa pamięci i ochrona przed usunięciem pochodzą wtedy z miejsca docelowego. BombVault najpierw sprawdza, czy logowanie miejsca docelowego otwiera repozytorium, i odmawia umieszczenia repozytorium append-only pod miejscem, które nie jest append-only.
- Naczynie pokazujące, jak pełne jest miejsce docelowe, ze znacznikiem, gdzie jest pełne, i liczbą tygodni, które zostały przy przyroście z ostatnich tygodni. Miejsce, które nie zgłasza rozmiaru, mówi **Rozmiar nieznany**.

Serwer odbiorczy, który uruchamia inna instancja Twojej grupy, pojawia się wśród dostawców jako **Z twojej grupy**; zobacz [Serwer odbiorczy](#receiving-server).

### Przenoszenie i usuwanie {#move-remove}

- **Przenieś** przenosi kopię jednej sekcji do innego miejsca docelowego, razem z całą historią. Wszystko zostaje w starym miejscu; BombVault niczego tam nie usuwa. Przenoszenie działa w tle i może trwać długo: około 9 minut na 100 GB między dyskami lokalnymi, godziny przez wolne łącze. Przegląd pokazuje je, póki trwa.
- **Usuń miejsce docelowe** oferuje dwie drogi. **Usuń miejsce docelowe, zachowaj kopie** przestaje tam zapisywać i zostawia kopie na miejscu; później możesz je odczytać w **Przywracanie, Ze starego folderu z kopiami**. **Usuń miejsce docelowe i skasuj tamtejsze kopie zapasowe** najpierw odczytuje, co tam leży, i to wymienia: które kopie znikną, które z nich nie istnieją nigdzie indziej, i które repozytoria zostają, bo ochrona przed usunięciem odmawia, elementy nadal tworzą do nich kopie wprost, inne miejsce docelowe lub sekcja trzyma tam swoje kopie albo nie dało się ich odczytać. Elementy, które straciłyby jedyną kopię, są wymienione osobno i zostają, chyba że zaznaczysz **Skasuj też te**. Potwierdzasz, wpisując nazwę miejsca docelowego, a usuwanie działa w tle.
- Usunięcie kopii w miejscu docelowym wymaga hasła logowania i zalogowania na samej instancji. Bez hasła miejsce docelowe nadal można usunąć, tylko jego kopii nie da się stąd skasować.

## Rozmieszczenie per element {#placement}

Karta **Harmonogram i miejsce docelowe** na karcie Ustawienia elementu ma rząd przycisków: **Lokalne** i po jednym przycisku na każde miejsce docelowe sekcji poza siedzibą. Zapalone przyciski dostają kopie elementu.

- Przy zapalonym **Lokalne** element jest zapisywany do repozytorium widocznego w **Zapisane na** i kopiowany do każdego innego zapalonego miejsca. Zgaś miejsce, a nie dostanie od tego elementu niczego nowego. Samo Lokalne nie kopiuje nigdzie, co pasuje do danych, które mają już drugą kopię, na przykład udziału leżącego na NAS.
- Przy zgaszonym **Lokalne** element jest zapisywany prosto do repozytorium bezpośredniego pierwszego zapalonego miejsca i stamtąd kopiowany do pozostałych zapalonych. Za pierwszym razem okno dialogowe tworzy to repozytorium bezpośrednie.
- Jeden przycisk pozostaje zapalony, bo kopia potrzebuje miejsca, dokąd trafi. Aby wyłączyć element z kopii, użyj **Nie kopiuj**.

Lokalizacja jest stała od pierwszej kopii elementu, bo BombVault nigdy nie przenosi kopii między repozytoriami; nazywa ją karta **Repozytorium** na karcie Ustawienia. Kopie mogą się zmieniać w każdej chwili. Miejsce, które przestaje dostawać element, zachowuje kopie, które ma, i przycina je do własnego przechowywania przy następnym przebiegu sekcji poza siedzibą. Z miejsc append-only nie można usuwać.

Pod rzędem karta mówi, dokąd element trafia i co faktycznie tam jest: ile lokalizacji go przechowuje, kiedy ostatnio widziano każde miejsce i czy spełnione jest 3-2-1. Lokalizacja to serwer z oryginalnymi danymi, każde miejsce poza siedzibą i każde repozytorium oznaczone **Poza obiektem**. BombVault sprawdza kopie i lokalizacje; nie sprawdza części „dwa nośniki” z 3-2-1.

### Ustawienia domyślne sekcji {#placement-defaults}

Karta **Miejsca docelowe według sekcji** na stronie Miejsca docelowe to ustawienie domyślne każdej sekcji. Na wykresie każda linia to kopia, a kliknięcie linii lub kropki włącza ją albo wyłącza; linie przerywane są wyłączone. **Wykres / Przełączniki** pokazuje to samo jako rzędy przycisków. Kopie obowiązują od razu każdy element bez własnego wyboru oraz foldery projektów stosów Compose. Lokalizacja obowiązuje nowy element przy jego pierwszej kopii; jej zmiana nie przenosi żadnych kopii. Przed zapisem BombVault wymienia każde miejsce, które zyskuje lub traci elementy, i ile migawek to oznacza.

Elementy, które tworzą kopie gdzie indziej niż ich sekcja, stoją w karcie **Własny wybór**, każdy z **Użyj ustawienia domyślnego**. Nowe kopie trafiają wtedy do miejsc docelowych sekcji, a wcześniejsze zostają tam, gdzie są.

Nowe miejsce docelowe poza siedzibą dostaje każdy element, który nie jest ustawiony na Lokalne. Okno, które je dodaje, mówi, ile to elementów i, tam gdzie wiadomo, ile historii.

### Repozytoria bezpośrednie

Wyłączenie Lokalne dla elementu, tak by domem stało się miejsce bez repozytorium bezpośredniego, otwiera okno z proponowaną lokalizacją obok niego, na przykład `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, i testem połączenia, który niczego nie tworzy. **Utwórz i używaj** tworzy repozytorium i kieruje do niego element. Repozytorium bezpośrednie przejmuje klucz, klasę pamięci, limity, ustawienie append-only i przechowywanie miejsca docelowego i zmienia się razem z nimi; karta Repozytoria pokazuje je tylko do odczytu. Gdy nowy klucz miejsca docelowego nie może go otworzyć, repozytorium bezpośrednie zachowuje klucz, który ma, i zapis o tym informuje. Element w repozytorium bezpośrednim jest stamtąd kopiowany do pozostałych zapalonych miejsc, nigdy do miejsca, do którego repozytorium należy. Jego migawki noszą tag `bv:direct`, a każdy inny przebieg przechowywania je zachowuje, więc repozytorium bezpośrednie, które utraciło powiązanie z miejscem, nigdy nie starzeje się według reguł lokalnych. B2 osiąga się przez jego punkt końcowy S3, z identyfikatorem klucza i kluczem aplikacji wpisanymi jako dane logowania S3; klucz ograniczony do własnego folderu miejsca nie sięga folderu obok, więc ogranicz klucz zamiast tego do folderu wyżej.

### Poza lokalizacją

Miejsce docelowe lub nazwane repozytorium można oznaczyć jako **Poza lokalizacją**. Zdalne zaczynają oznaczone; wyłącz to dla rest-servera w tym samym budynku. Oznaczenie tylko liczy lokalizacje i 3-2-1 na kartach. Nie zmienia żadnej kopii.

Replika ZFS to kopia poza lokalizacją, ale nie kopia zapasowa z historią. Trzyma tylko migawki, które ustawisz, a stare wersje i pojedyncze pliki nadal pochodzą z kopii zapasowych. Idzie z jednego serwera ZFS prosto na drugi, przez SSH do serwera ZFS albo do punktu odbioru sparowanej instancji. Grupa parowania niesie tylko prośbę. Zobacz [Replika](zfs-datasets.md#replica).

### Po odbudowie {#after-a-rebuild}

Wybory dotyczące kopii żyją we własnych ustawieniach BombVault. Po odbudowie przez **Odkryj kopie zapasowe** lub Kreator awaryjny bez przywróconych ustawień znikają, a kopiowanie wszystkiego wysłałoby do B2 znów elementy, które zostały pominięte. Kopie poza siedzibą każdej odbudowanej sekcji są więc wstrzymane. Przegląd to pokazuje, a karta **Miejsca docelowe według sekcji** oferuje **Potwierdź domyślne** z podglądem tego, co skopiuje następny przebieg, i nazwami w kopiach, które nie mają elementu, które możesz tam pominąć. Pauzę kończy tylko potwierdzenie; import pliku ustawień przywraca reguły i ustawienia domyślne, ale jej nie kończy.

## Zdalne repozytoria podstawowe {#remote-primary-repositories}

Własna ścieżka sekcji nie jest ograniczona do folderu lokalnego: skieruj ją prosto na zdalny zasób restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`), a BombVault będzie tworzyć kopię prosto tam, bez osobnej kopii lokalnej i bez kroku kopiowania. To inny układ niż kopie poza siedzibą powyżej: tam repozytorium lokalne jest podstawowe, a repozytorium poza siedzibą to jego archiwum best-effort; tutaj zdalne repozytorium **jest** podstawowe i jest jedyną kopią, chyba że sekcję kopiujesz też do innego miejsca docelowego.

Ścieżkę każdej sekcji (Containers, Maszyny wirtualne, Flash, Autokopia, Foldery, Zbiory danych ZFS) ustawia się na stronie miejsca docelowego **Folder lokalny**: **Zmień** obok sekcji w jego karcie **Połączenie** otwiera okno z przełącznikiem **Lokalne / Zdalne**:

- **Lokalne** pokazuje znany wybierak folderów.
- **Zdalne** zastępuje go zwykłym polem URL i ustawieniami połączenia, jakich używa miejsce docelowe poza siedzibą, skonfigurowanymi tym razem dla tego repozytorium podstawowego. Stamtąd dostajesz:
    - **Test połączenia** z żywą ścieżką, zanim na niej polegasz.
    - **Limity przepustowości** (wysyłanie/pobieranie), by zaplanowana kopia do zdalnego repozytorium podstawowego nie nasycała Twojego WAN: te same flagi restic `--limit-upload`/`--limit-download`, których używają kopie poza siedzibą, zastosowane do samej kopii zapasowej.
    - **Ochronę append-only**, weryfikowaną tym samym aktywnym testem manipulacji (prawdziwa sonda DELETE po drugiej stronie), jaki dostają miejsca poza siedzibą. Gdy jest włączona, BombVault odmawia przycinania samego repozytorium, bo nie ma za nim osobnej kopii lokalnej, a dane logowania na tej maszynie nie mogą móc usunąć jedynej kopii.
    - **Alarm budżetu wzrostu**, próbkowany z tego samego trendu rozmiaru repozytorium, który śledzi już karta Pamięć.

Nic z tego nie jest wymagane: ręcznie wpisana zdalna ścieżka bez zapisanych ustawień bezpieczeństwa tworzy kopie bez limitu, da się ją przycinać i nie ma alarmu budżetu. Ustawienia bezpieczeństwa są na wypadek, gdy chcesz ochrony, jaką dostaje kopia poza siedzibą, bez osobnego miejsca docelowego tylko po to.

!!! note "Dane logowania do chmury i REST są wspólne"
    Zdalne repozytorium podstawowe loguje się współdzielonymi danymi logowania do chmury w **Ustawienia, Miejsca docelowe**; nie ma osobnego magazynu danych logowania dla repozytoriów podstawowych.

### SMB i WebDAV bez montowania na hoście {#smb-webdav}

Karta **Off-site (rclone)** w **Ustawienia, Miejsca docelowe** ma formularz dla udziału Windows lub Samba oraz dla serwera WebDAV (Nextcloud, ownCloud, SharePoint lub inny). Wpisz krótką nazwę, host i udział (SMB) albo URL i typ serwera (WebDAV), użytkownika i hasło, a BombVault zapisze za Ciebie sekcję rclone. rclone zaciemnia hasło sam, zanim zostanie zapisane; dodanie pozycji o nazwie, która już istnieje, zastępuje tę sekcję zamiast dodawać drugą. **Dodaj miejsce docelowe** oferuje SMB i WebDAV jako dostawców z tymi samymi polami.

Formularz odpowiada gotową lokalizacją, na przykład `rclone:nas:backups`. Użyj jej jako ścieżki sekcji lub miejsca docelowego i dopisz podfolder, jeśli chcesz (`rclone:nas:backups/bombvault`). Udział to pierwszy segment ścieżki, a nie część nazwy.

To lepsza droga niż montowanie udziału na Unraid: restic odradza trzymanie repozytorium na zamontowanym udziale CIFS, a tutaj nic nie jest montowane. NFS nie ma w formularzu, bo ani restic, ani rclone nie mają backendu NFS; dla NFS zamontuj eksport na hoście i użyj go jako folderu lokalnego.

## Ochrona przed usunięciem (append-only)

Włącz ochronę przed usunięciem dla miejsca docelowego poza siedzibą, aby ransomware lub przejęty host nie mogły usunąć ani przepisać Twoich kopii zapasowych. Druga strona (`restic/rest-server` działający w trybie `--append-only`) ją **egzekwuje**. BombVault ją tylko **weryfikuje** i nigdy nie pokazuje zieleni na samym zapewnieniu konfiguracji. Folder na tym serwerze nie może odmówić usunięcia, bo kto kontroluje BombVault, kontroluje też folder, więc miejsca lokalne nie oferują tego przełącznika.

Najszybsza droga do kopii append-only to sparowany BombVault z [serwerem odbiorczym](#receiving-server): uruchamia rest-server za Ciebie, w trybie append-only.

!!! note "Udane usunięcie w `/locks/` jest oczekiwane"
    Append-only nie oznacza, że nic nigdy nie może zostać usunięte. restic musi brać i zwalniać własne blokady, więc `/locks/` pozostaje z założenia zapisywalne i usuwalne. Migawek i danych za nimi, których szuka ransomware, nie da się usunąć. Jeśli sam testujesz drugą stronę, usunięcie, które udaje się w `/locks/`, jest poprawnym zachowaniem, a nie dziurą w ochronie.

!!! warning "Repozytoria append-only nigdy nie są przycinane z tej maszyny"
    Miejsce docelowe append-only nigdy stąd nie przycina starych migawek; przycinanie to zadanie drugiej strony. Ustaw dla niego **alarm budżetu wzrostu**, żeby dostać ostrzeżenie, zanim rozmiar repozytorium wymknie się spod kontroli.

## Tamper test

BombVault okresowo dowodzi gwarancji append-only, faktycznie próbując usunąć coś w repozytorium poza siedzibą, na nieistniejącym obiekcie. **Przetestuj ochronę przed usunięciem teraz** na stronie miejsca docelowego uruchamia to na żądanie, a **Sprawdzaj ochronę przed usunięciem** w **Ustawienia, Weryfikacja, Automatyczne kontrole** uruchamia to według harmonogramu.

- **Odrzucone** oznacza chronione.
- **Zaakceptowane** oznacza niechronione.
- **Niejednoznaczny** wynik (serwer nieosiągalny, błąd uwierzytelniania) nigdy nie odwraca zapisanego werdyktu.

Prawdziwe przejście z chronionego na niechronione wywołuje pojedynczy alert.

## Próby DR

BombVault oferuje dwa poziomy dowodu, że Twoje kopie zapasowe da się naprawdę przywrócić, a nie tylko że istnieją.

- **Próby weryfikacji przywracania (lokalne).** BombVault okresowo uruchamia `restic check --read-data-subset` (ograniczone, nigdy pełne przywracanie zapychające dysk). Rytm ustawia się w **Ustawienia, Weryfikacja, Automatyczne kontrole**, a karta **Sprawdź kopie zapasowe** na tym samym kafelku pokazuje wynik.
- **Próbne przywracanie (poza siedzibą).** BombVault przywraca prawdziwy cel z repozytorium poza siedzibą do odizolowanej piaskownicy, weryfikuje go plik po pliku i bajt po bajcie, po czym sprząta. To dowodzi, że potrafisz odzyskać dane spoza siedziby, a nie tylko że repozytorium odpowiada.

Karta **Stan ochrony** w przeglądzie podsumowuje to dla każdej sekcji w czterech kontrolach: aktualna kopia na tym serwerze, kopia poza siedzibą, ochrona przed usunięciem tam i próbne przywracanie stamtąd. Kliknięcie sekcji pokazuje szczegóły, a karta robi się zielona tylko na zweryfikowanych faktach.

## Parowanie instancji {#pairing}

Kilka instancji BombVault współpracuje jako jedna grupa: pokazują nawzajem swój stan ochrony, przejmują dla siebie role i mogą przechowywać swoje klucze. Instancja dołącza do grupy za pomocą dwunastu słów.

Na pierwszej instancji otwórz **Ustawienia, Parowanie** i naciśnij **Wygeneruj frazę**. Trzy karty kroków pokazują, jak to przebiega. Dwanaście słów pojawia się w oknie z przyciskiem **Kopiuj**. Na każdej innej instancji otwórz to samo miejsce, naciśnij **Wpisz frazę** i wklej lub wpisz je, albo naciśnij **Wklej** w tym oknie. Słowo spoza listy jest wskazywane z pozycją podczas pisania, a ostatnie słowo niesie sumę kontrolną, więc błędnie wpisane lub zamienione słowo zostaje wychwycone, zanim cokolwiek się sparuje. Generuj frazę tylko na jednej instancji: dwie instancje, które obie ją wygenerują, tworzą dwie osobne grupy. Jeśli przez minutę nikt się nie pojawi, strona oferuje dwa wyjścia: pokazać słowa ponownie, aby wpisać je tam, albo wpisać słowa drugiej instancji i dołączyć do jej grupy jednym krokiem. Parowanie działa bez hasła logowania, ale ustaw je: bez niego każdy, kto może otworzyć interfejs webowy, może odczytać słowa i przez grupę prosić o role, które wydają hasło restic instancji w niej. Karta parowania o tym mówi, dopóki hasło nie zostanie ustawione. Z hasłem ponowne pokazanie frazy o nie prosi. **Opuść grupę** wyjmuje instancję z powrotem.

Każdy, kto zna słowa, może dołączyć do grupy, więc traktuj je jak hasło.

**Jak członkowie się docierają.** Każda instancja poznaje swój adres w sieci z Twojej przeglądarki w chwili logowania; widać go w **Znane adresy** na stronie parowania razem z tymi, które instancja znalazła sama; popraw go tam, jeśli przed nią stoi reverse proxy lub nietypowy port. W tej samej sieci członkowie ogłaszają ten adres multicastem i rozmawiają bezpośrednio, a tam, gdzie multicast nie może przekroczyć sieci kontenerów, jak domyślny bridge Dockera, instancja zamiast tego przeszukuje własną podsieć w poszukiwaniu pozostałych podpisanym wywołaniem, na które może odpowiedzieć tylko członek grupy, więc parowanie kończy się w kilka sekund bez relay. Jeśli nic się nie pojawi, **Nie możesz jej znaleźć?** przyjmuje jeden adres ręcznie, dla innej podsieci lub niestandardowego portu. Instancje w różnych sieciach idą przez relay, wybierany na tej samej stronie; każda z czterech form ma obrazek:

- **Relay projektu** (domyślny): `parleyport.halleluja.design`, relay, którego używa też KnightLoader. Nic do ustawiania.
- **Własny relay**: kontener [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) z Unraid Community Apps (także w **Ustawienia, Aplikacje**) albo jedna z Twoich instancji, która jest już osiągalna z zewnątrz, z włączonym **Działaj jako relay**. Ta instancja odpowiada wtedy pod `/relay/connect` na własnym adresie, za reverse proxy i certyfikatem, które już ma, i wpuszcza tylko Twoją grupę. Wpisz adres relay na każdej instancji, która ma go używać.
- **Brak relay**: członkowie znajdują się automatycznie w tej samej sieci i nigdzie indziej.

**Co widzi relay.** Każde wywołanie między członkami jest zapieczętowane AES-256-GCM kluczem wyprowadzonym z dwunastu słów, a ten klucz nigdy nie opuszcza Twoich instancji. Relay poznaje skrót grupujący połączenia, dla której instancji jest wiadomość, jak jest duża i kiedy przechodzi. Bezpośrednie wywołanie w sieci lokalnej jest zapieczętowane tak samo i dodatkowo podpisane, więc nic nie zależy od certyfikatu samopodpisanego, który serwuje instancja. Wywołania, które coś wydają (odpowiedzi na role, hasło restic, loginy odbiorców, depozyty kluczy), są dodatkowo podpisane kluczem członka, który je wysyła, przypiętym przy pierwszym spotkaniu; zobacz [Model bezpieczeństwa](configuration.md#security-model).

**Co podróżuje przez grupę.** Stan ochrony na stronie Instancje, prośba o sprawdzenie jednej sekcji teraz, prośby o role i odpowiedzi na nie, oferty pamięci oraz to, czego odbiorca lub pobierający potrzebuje po zezwoleniu na swoją rolę: lokalizacje repozytoriów drugiej instancji i jej hasło restic. Dane kopii nigdy; nadal idą prosto do backendów restic. APP_KEY też nie: hasło restic otwiera repozytoria tej instancji i nic więcej, nie jej przechowywane sekrety, sesje ani kody odzyskiwania.

**Wpisy sprzed parowania.** Instancje dodane tokenem floty oraz otrzymane repozytoria i źródła pobierania ustawione za pomocą APP_KEY drugiej instancji zostają po aktualizacji i są oznaczone **Sparuj ponownie**. Nadal działają: przy pierwszym uruchomieniu BombVault zastępuje każdy zapisany APP_KEY hasłem restic z niego wyprowadzonym. Sparuj obie instancje, potem edytuj wpis i wybierz jego instancję. Do tego czasu takie wpisy stoją w karcie **Bez instancji** na stronie Instancje.

Jedyne miejsce, które nadal przyjmuje APP_KEY ręcznie, to [Ze starego folderu z kopiami](#restore-from-another-bombvault-repo), na wypadek gdy druga instancja zniknęła i nie może odpowiedzieć w grupie.

## Instancje i role {#roles}

Strona **Instancje** (kafelek ustawień, który można przypiąć do paska bocznego) pokazuje grupę jako sieć tego, kto co robi dla kogo, kartę tej instancji i kartę każdego innego członka, telefony z Androidem i serwery ZFS. **Sparuj instancję** unosi się w prawym dolnym rogu. To, które części są włączone, ustawia się w **Ustawienia, Instancje, Wiele serwerów**: **Instancje**, **Odbiornik** i **Pobieranie**.

Każda karta pokazuje stan ochrony członka i jego role jako kafelki. Rola to coś, co jedna instancja robi dla drugiej:

- **Odbiornik** trzyma kopie tego serwera poza siedzibą w rest-serverze w trybie append-only. Ten serwer wysyła po każdej kopii zapasowej i nie może tam niczego usunąć, nawet jeśli ktoś go przejmie.
- **Pobierający** pobiera kopie zapasowe tego serwera według własnego harmonogramu. Ten serwer nie musi być do tego włączony.
- **Serwer ZFS** przyjmuje repliki ZFS tego serwera. Każdy element ZFS wybiera go w **Replika** w swoich ustawieniach.
- **Przechowawca klucza** trzyma zapieczętowaną kopię zestawu odzyskiwania tego serwera. Zobacz [Przechowawca klucza](#key-keeper).

Włączenie roli wysyła prośbę, a nic się nie zaczyna, dopóki ktoś na drugiej instancji nie naciśnie **Zezwól**. Prośba pojawia się na karcie tej instancji jako odznaka, a jej **Szczegóły** wymieniają ją z **Zezwól** i **Odrzuć**; zezwoloną rolę można tam w każdej chwili cofnąć. Linia przerywana w sieci to rola, która jeszcze czeka. Instancja w BombVault 9.x nie może odpowiedzieć na prośbę, więc rola jest tam aktywna, gdy tylko ją włączysz, a prośba od instancji 9.x czeka tutaj na **Zezwól**.

**Szczegóły** na karcie otwierają resztę: otrzymane repozytoria i źródła pobierania tego członka, oferty pamięci, monitorowanie, **Usuń** i **Zapomnij klucz**.

## Odbiornik i pobierający {#receiver}

![Otrzymana kopia poza siedzibą w szczegółach instancji, obserwowana tylko do odczytu, z kontrolą spójności uruchomioną na tym sprzęcie.](assets/screenshots/receiver.png)

*Otrzymana kopia poza siedzibą w szczegółach instancji, obserwowana tylko do odczytu, z kontrolą spójności uruchomioną na tym sprzęcie.*

Na maszynie, która **odbiera** kopie append-only od innego BombVault, otrzymane repozytoria dają niezależne monitorowanie tylko do odczytu na sprzęcie odbierającym, więc cichy błąd po drugiej stronie nie przechodzi niezauważony. Włącz **Odbiornik** w **Ustawienia, Instancje**, potem otwórz **Szczegóły** instancji wysyłającej na stronie Instancje i dodaj otrzymane repozytorium (tylko do odczytu, otwierane hasłem restic instancji wysyłającej, które dostaje ona przez [grupę parowania](#pairing)). Dostajesz:

- **Inwentarz według źródła**, więc widzisz dokładnie, które kontenery, maszyny wirtualne i zestawy folderów dotarły.
- **Ostatnio otrzymane** per źródło, więc wiesz, jak świeże jest każde.
- **Niezależny `restic check`** uruchomiony na sprzęcie odbierającym, więc spójność jest weryfikowana tam, gdzie dane faktycznie leżą, a nie tylko u nadawcy.
- **Wyłącznik bezpieczeństwa:** alert, gdy źródło przestaje wysyłać w ustawionym oknie.
- **Alerty spójności:** alert, gdy kontrola po stronie odbierającej zawiedzie.

Odbiornik jest ściśle tylko do odczytu. Nigdy nie zapisuje do otrzymanego repozytorium, więc nigdy nie może złamać gwarancji append-only, na której polega nadawca.

Pobierający działa w drugą stronę: przy włączonym **Pobieranie** instancja wczytuje kopie zapasowe innego członka do własnego repozytorium według własnego harmonogramu. Jej źródła stoją też w szczegółach tego członka.

### Serwer odbiorczy {#receiving-server}

Maszyna odbierająca może też uruchomić rest-server, na który kopiują pozostałe. Ustawia się go przy pierwszym zezwoleniu na prośbę **Odbiornik**, która o to prosi: okno mówi, że serwer potrzebuje serwera odbiorczego, i pyta o folder na udziale, z **Nowy folder** do utworzenia jednego, oraz port (8000, chyba że używa go inny kontener). BombVault wtedy:

1. odmawia, jeśli kontener o nazwie `rest-server` już istnieje albo port zajmuje inny kontener;
2. pobiera `restic/rest-server` i uruchamia go przez gniazdo Docker w trybie append-only z prywatnymi repozytoriami i plikiem logowania w folderze;
3. zapisuje jego szablon Unraid na flashu, dzięki czemu kontener można dalej edytować w zakładce Docker, albo oferuje szablon do pobrania, gdy flash jest poza zasięgiem;
4. uruchamia na nim test manipulacji i pokazuje, czy serwer odrzuca usuwanie.

Instancje Twojej grupy znajdują potem serwer wśród dostawców w **Z twojej grupy**, nazwany od maszyny odbierającej, a rola **Odbiornik** proponuje go jako bezpieczny wybór. Każda instancja dostaje własny login przy pierwszym użyciu serwera i zapisuje tam tylko do własnego folderu. Przy włączonym **Odbiornik** w **Ustawienia, Instancje** **Serwer odbiorczy** na karcie tej instancji otwiera serwer ponownie. Jego okno wymienia loginy w **Partnerzy z loginem**, a **Odwołaj login** jeden zabiera; to, co ta instancja już skopiowała, zostaje w folderze. Konfiguracja tworzy też jeden **Login spoza grupy**, którego hasło okno pokazuje raz.

Instancja, która dociera do maszyny odbierającej tylko przez relay, nie może użyć serwera, bo relay nie niesie kopii zapasowych. Najpierw dodaj adres maszyny odbierającej na stronie parowania. Gdy BombVault działa na własnym adresie IP (na przykład na br0), wypełnij **Adres dla partnerów**, bo serwer nasłuchuje na adresie hosta.

## Pełny przykład: dwie maszyny Unraid, od początku do końca

Wszystko powyżej opisuje części. To jedna kompletna konfiguracja, bo części łatwiej złożyć, gdy raz widziało się je złożone.

Dwie maszyny: **TOWER** uruchamia kontenery i wypycha kopie; **VAULT** je odbiera i egzekwuje append-only. Podstaw własne nazwy, adresy i ścieżki udziałów.

**1. Sparuj obie maszyny.** Na TOWER otwórz **Ustawienia, Parowanie**, naciśnij **Wygeneruj frazę** i wpisz dwanaście słów na VAULT w **Wpisz frazę**. Najpierw ustaw hasło logowania na obu.

**2. Na TOWER poproś VAULT o rolę odbiornika.** Na stronie Instancje włącz **Odbiornik** na karcie VAULT, wybierz sekcje i **serwer odbiorczy VAULT** jako miejsce, a potem wyślij prośbę.

**3. Na VAULT zezwól.** Strona Instancje na VAULT pokazuje prośbę na karcie TOWER. Naciśnij **Zezwól**. Za pierwszym razem VAULT pyta o folder dla swojego serwera odbiorczego, na przykład `user/appdata/rest-server`, uruchamia rest-server w trybie append-only i odpala na nim test manipulacji.

**4. Z powrotem na TOWER.** Od tej chwili TOWER kopiuje na VAULT po każdej kopii zapasowej, a nowe miejsce docelowe pojawia się na stronie Miejsca docelowe w TOWER z jego ochroną przed usunięciem.

Co odpowiada test manipulacji:

| Wynik | Co się stało |
| --- | --- |
| **chronione** | VAULT odrzucił usunięcie. To jedyny stan zaliczający. |
| **niechronione** | VAULT przyjął usunięcie. Brakuje `--append-only` albo je usunięto. |
| **niejednoznaczny** | Ani jedno, ani drugie. Zwykle URL nie jest tym, którego używa sam restic, albo zmieniły się dane logowania. Nic nie jest zapisywane i żaden alert się nie uruchamia. |

**5. Obserwuj, co przychodzi, jeśli chcesz.** Włącz **Odbiornik** w **Ustawienia, Instancje** na VAULT, otwórz **Szczegóły** TOWER i dodaj otrzymane repozytorium. VAULT dostaje hasło restic TOWER przez grupę; nikt nie wpisuje klucza.

**6. Zrób to wzajemnie.** Powtórz kroki w drugą stronę: VAULT prosi TOWER, aby był jego odbiornikiem. Każda maszyna egzekwuje wtedy append-only dla drugiej i żadna nie może usunąć kopii zapasowych drugiej.

Rest-server uruchomiony ręcznie też działa: dodaj go przez **Dodaj miejsce docelowe** jako rest-server, z użytkownikiem i hasłem z jego `.htpasswd`, i włącz ochronę przed usunięciem. Zostaw `--append-only` w jego opcjach; bez tego druga strona znów jest zwykłym udziałem.

!!! warning "Ścieżka lokalna to ścieżka wewnątrz kontenera"
    Tam, gdzie BombVault pyta o folder na tym serwerze, wpisz go względem montowania hosta, na przykład `user/appdata/rest-server/bombvault-containers`, a nie `/mnt/user/appdata/...`. BombVault działa w kontenerze, w którym `/mnt` hosta jest zamontowany gdzie indziej; bezwzględna ścieżka hosta tam nie istnieje. Jeśli wkleisz taką, BombVault podpowie, jakiej ścieżki względnej użyć.

## Kreator awaryjny {#emergency}

**Kreator awaryjny** przywraca serwer, którego już nie ma albo który został postawiony od nowa. Startuje na stronie **Przywracanie** w **Awaria** i z **Przywróć z kopii zapasowej** na stronie powitalnej nowej instalacji. Przebiega w czterech krokach: **Dostęp**, **Ustawienia**, **Wybór** i **Przywracanie**.

1. **Dostęp.** Pokaż BombVault klucz starych kopii zapasowych, na jeden z trzech sposobów:
    - **Wgraj zestaw odzyskiwania**: plik pobrany podczas konfiguracji. Zawiera klucz i każde miejsce docelowe, a BombVault uzupełnia resztę na jego podstawie. Zestaw zapieczętowany age prosi o tożsamość, dla której został zapieczętowany; jest używana raz i nie jest zapisywana.
    - **Przez grupę**: instancja Twojej grupy trzyma klucz jako depozyt. Potrzebujesz dwunastu słów grupy, swojego kodu odblokowania i kogoś przy tej instancji. Zobacz [Przechowawca klucza](#key-keeper).
    - **Wpisz klucz**: `APP_KEY` starego serwera i miejsce, gdzie są jego kopie.

    BombVault próbuje potem klucza na każdym miejscu docelowym i mówi dla każdego, czy jest czytelne, nie znalezione, zablokowane innym kluczem, nieosiągalne albo nadal bez logowania. Nic nie zostało jeszcze zapisane, a klucz zostaje w pamięci przez ograniczony czas. **Przejmij klucz** sprawia, że ten serwer odtąd zapisuje kopie tym kluczem; **Cofnij przejęcie** wraca do jego własnego klucza, dopóki nie zadziała pierwsza kopia zapasowa. Serwer, który ma już własne kopie, odmawia przejęcia i odsyła do [Ze starego folderu z kopiami](#restore-from-another-bombvault-repo). **Zapisz dane dostępowe** zapisuje logowania, które zestaw niesie dla zdalnych miejsc docelowych, i pyta przed zastąpieniem logowań, które ten serwer już ma.
2. **Ustawienia.** **Przywróć ustawienia** odtwarza własne ustawienia BombVault z najnowszej autokopii: miejsca docelowe, dane logowania, harmonogramy i reguły. BombVault uruchamia się ponownie, aby je zastosować, przez gniazdo Docker, więc żywa baza ustawień nigdy nie jest nadpisywana przy otwartym uchwycie, a kreator potem sam idzie dalej. Logowanie tego serwera zostaje: nadal logujesz się hasłem, którego używasz teraz. Sekrety zapisane przez stary serwer, których nie da się przejąć, są wymienione, żeby wpisać je ponownie.
3. **Wybór.** BombVault przegląda Twoje kopie zapasowe i wymienia znalezione kontenery, maszyny wirtualne, zestawy folderów i zbiory danych ZFS. Najnowsza migawka jest wstępnie wybrana wszędzie. To, co ten serwer nadal ma, startuje wyłączone, bo przywracanie by to zastąpiło. Zestawy folderów po odbudowie nie wiedzą już, skąd pochodziły, więc trafiają do wybranego folderu.
4. **Przywracanie.** Wszystko wybrane wraca naraz, zatrzymane, jeśli chcesz uruchomić każdy kontener i maszynę samodzielnie, co jest najbezpieczniejsze po całkowitej utracie. Potem pobierz nowy zestaw odzyskiwania: stary należy do poprzedniej instalacji.

!!! tip "Planowana migracja kontra awaria"
    Kreator awaryjny przywraca własne ustawienia BombVault z kopii zapasowej. Przy *planowanej* przeprowadzce na nową maszynę możesz zamiast tego przenieść konfigurację bezpośrednio kartą **Eksport / import ustawień** (przenośny plik JSON). Zobacz [Konfiguracja](configuration.md#portable-settings-export-and-import).

### Ze starego folderu z kopiami {#restore-from-another-bombvault-repo}

**Ze starego folderu z kopiami** w **Inne źródła** na stronie Przywracanie otwiera repozytorium *innej* instancji BombVault (udział zamontowany pod `/mnt` albo adres zdalny) za pomocą zestawu odzyskiwania tej instancji lub jej wpisanego `APP_KEY`, w jednorazowej sesji tylko do odczytu. Przeglądaj kontenery, maszyny wirtualne, zestawy folderów i zbiory danych ZFS, które tam leżą, wybierz migawkę i przywróć ją, a przywrócony obiekt stanie się zwykłym lokalnym kontenerem, maszyną lub zestawem folderów. Nic nie jest nigdy zapisywane do drugiego repozytorium, a Twoje ustawienia kopii pozostają nietknięte (sesja żyje w pamięci i wygasa sama). Przeniesienie kontenera z serwera A na serwer B nie oznacza przekierowywania ustawień i cofania ich potem. To okno jest jednorazowe: otwiera sesję, przywraca to, co wybierzesz, i zapomina drugą instancję. Jeśli zamiast tego chcesz stałego układu, w którym ta maszyna według harmonogramu pobiera migawki innej instancji do własnego repozytorium, to rola **Pobierający** na stronie Instancje.

Kontener, którego sieć nie istnieje na tym serwerze, na przykład sieć Unraid `br0` na zwykłym hoście Docker, pokazuje pod swoim wierszem wybierak sieci. BombVault tworzy go w wybranej sieci razem z jego pozostałymi sieciami. Stały IP i adres MAC należały do starej sieci i są odrzucane, więc nowa sieć je przydziela.

## Przechowawca klucza {#key-keeper}

Przechowawca klucza to instancja Twojej grupy, która trzyma zapieczętowaną kopię zestawu odzyskiwania tego serwera. Sama nie może jej otworzyć. Jeśli ten serwer kiedykolwiek przepadnie, nowa instalacja, która dołącza do Twojej grupy, prosi przechowawcę o kopię i otwiera ją Twoim kodem odblokowania.

1. Na stronie Instancje włącz **Przechowawca klucza** na karcie instancji, która ma go trzymać, i wybierz **Złóż depozyt**. Złożenie depozytu wymaga hasła logowania.
2. Ten serwer pokazuje **Kod odblokowania** raz: dwadzieścia liter i cyfr w czterech grupach po pięć. Zapisz go i potwierdź **Mam zapisany kod**. Ten serwer nie zapisuje kodu, tylko to, czego potrzebuje, by zapieczętować dla niego depozyt.
3. Przechowawca trzyma teraz depozyt, a ten serwer odnawia go, gdy zestaw się zmienia. **Depozyt klucza** na karcie tej instancji wymienia **Depozyt tego serwera** z każdym przechowawcą i jego stanem oraz **Depozyty przechowywane na tym serwerze** dla pozostałych. **Utwórz nowy kod** zastępuje kod; każdy przechowawca przechodzi na nowy, gdy dostanie nowy depozyt.

Tego dnia potrzebujesz trzech rzeczy: dwunastu słów swojej grupy, kodu odblokowania i dostępu do interfejsu przechowawcy, gdzie ktoś musi nacisnąć **Zezwól**. W Kreatorze awaryjnym wybierz **Przez grupę**, wskaż depozyt, poproś o niego i wpisz kod odblokowania. Kod jest używany na nowym serwerze i nie jest wysyłany do przechowawcy. U przechowawcy prośba pojawia się w **Depozyt klucza** z informacją, kto prosi, czy ta instancja była tam kiedykolwiek widziana i jak często prosiła; **Zezwól** wydaje zapieczętowaną kopię, a zezwolenie wymaga hasła logowania. Nowa instalacja ma 30 minut na odebranie jej po zezwoleniu, prośba, na którą nikt nie odpowie, wygasa po dobie, a instancja, która prosi zbyt często, musi poczekać godzinę.

Bez kodu nikt nie może otworzyć kopii, przechowawca też nie. Jeśli kod zginął, utwórz nowy, póki ten serwer jeszcze działa; jeśli serwera już nie ma, nadal działają zestaw odzyskiwania i wpisany APP_KEY. Przechowawca może w każdej chwili usunąć depozyt, a właściciel musi wtedy poprosić ponownie.

## Zestaw odzyskiwania {#recovery-kit}

To element, który umożliwia odzyskiwanie po awarii nawet wtedy, gdy nie działa żaden BombVault.

**Pobierz zestaw odzyskiwania** w karcie **Na wypadek awarii** w **Ustawienia, Przywracanie** daje **klucz**, **wyprowadzone hasło restic**, każde **miejsce docelowe** z repozytorium każdej sekcji, logowania, których nowa instalacja potrzebuje, by do nich dotrzeć, oraz **dokładne polecenia restic**, dzięki czemu możesz przywracać prosto za pomocą restic CLI na dowolnej maszynie. Wymaga hasła logowania. Zestaw drukuje klucz także przy wyłączonym szyfrowaniu, bo definicje kontenerów i maszyn wirtualnych są nim zawsze pieczętowane. Po przejęciu drukuje przejęty klucz kopii zapasowych i APP_KEY tego serwera, którego nowa instalacja potrzebuje tylko do odtworzenia ustawień. Karta **Zapisz swój zestaw odzyskiwania** w przeglądzie przypomina o tym, dopóki go nie zapiszesz.

!!! danger "Przechowuj zestaw odzyskiwania poza serwerem"
    Zestaw zawiera sekret, który odszyfrowuje Twoje kopie zapasowe. Trzymaj go w bezpiecznym miejscu, oddzielnie od serwera (menedżer haseł, wydruk w sejfie). Jeśli stracisz zarówno BombVault, jak i `APP_KEY`, nie mając zestawu odzyskiwania ani depozytu klucza, zaszyfrowane kopie zapasowe nie dadzą się odzyskać.

!!! warning "Najnowsza migawka nie zawsze jest tą do przywrócenia"
    Od restic 0.17 `restic snapshots` pokazuje rozmiar każdej migawki. Po utracie danych najnowsza migawka może być tą opróżnioną, więc nie przywracaj migawki znacznie mniejszej od poprzednich. Po ransomware może to być zaszyfrowana, w zwykłym rozmiarze. Jeśli BombVault nadal działa, najpierw zajrzyj na stronę **Anomalie**: wskazuje ostatnią dobrą kopię, a okno przywracania zaznacza ją z góry. Przywracanie nie potrzebuje żadnych danych o anomaliach z BombVault, a wstrzymanie przechowywania zawsze tylko zachowuje więcej migawek.

### Pieczętowanie zestawu

Jeśli włączono **Szyfruj eksporty za pomocą age** w **Ustawienia, System**, zestaw też jest nim pieczętowany i pobiera się jako `bombvault-recovery-kit.md.age`. Ma postać ASCII-armor, a nie binarną, więc nadal jest zwykłym tekstem: wklejenie go do menedżera haseł albo wydrukowanie działa dokładnie jak wcześniej, tylko zawartość jest nieczytelna bez Twojego klucza.

!!! warning "Nie trzymaj klucza age w zestawie"
    Do otwarcia zapieczętowanego zestawu potrzebujesz swojego **prywatnego** klucza age. Trzymaj go w miejscu, które nie zależy od samego zestawu, inaczej będziesz mieć do odzyskania dwie rzeczy zamiast jednej. Pieczętowanie się opłaca, gdy zestaw leży w miejscu, którego w pełni nie kontrolujesz (współdzielony menedżer haseł, notatki w chmurze, wydruk w biurze); zestaw we własnym sejfie jest już chroniony przez sejf.

    Przy włączonym szyfrowaniu i braku skonfigurowanego użytecznego odbiorcy pobranie jest od razu odrzucane. BombVault nigdy nie wraca do wydania klucza głównego tekstem jawnym.

### Gdy zestawu nie ma pod ręką

Hasło nie jest nigdzie zapisane, jest **wyliczane** z klucza kopii zapasowych. Mając klucz i powłokę, możesz je odtworzyć samodzielnie:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

To HMAC-SHA256 po stałym ciągu `bombvault:restic-repo`, z surowymi bajtami szesnastkowego `APP_KEY` jako kluczem, wypisany jako 64 małe znaki szesnastkowe. Ta sama wartość jest w zestawie, wymieniona jako wyprowadzone hasło restic; to na dzień, gdy zestaw leży tam, gdzie Cię nie ma. Na serwerze, który przejął kopie innego, użyj przejętego klucza kopii zapasowych, czyli dawnego APP_KEY tamtego serwera.

!!! warning "Dla otrzymanego repozytorium użyj klucza instancji WYSYŁAJĄCEJ"
    Repozytorium, które trafiło tu przez kopię poza siedzibą, utworzyła maszyna, która je wysłała, swoim **własnym** kluczem. Wyprowadzenie z klucza maszyny odbierającej daje hasło, które restic odrzuca, co wygląda dokładnie jak uszkodzone repozytorium, choć nim nie jest. To zwykły powód, dla którego `restic check` na otrzymanym repozytorium wciąż pyta o hasło.

Ponieważ definicje odzyskiwania znajdują się **wewnątrz** każdego repozytorium (`<repo>/def`, `<repo>/vm-def`), skopiowany folder repozytorium jest w pełni samowystarczalny, więc zestaw plus repozytorium to wszystko, czego potrzebuje przywracanie na goły metal.

## Odzyskiwanie zrzutu bazy danych {#database-dumps}

Zrzut bazy danych jest osobnym punktem przywracania w repozytorium kontenerów, z etykietą `dbdump:<container>` i jednym plikiem, `/dbdump/<container>.sql`. BombVault wypisuje je, pobiera i importuje na karcie **Kopie zapasowe** kontenera; poniżej te same kroki z samym resticiem, na dzień, w którym BombVaulta nie ma.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Etykiety `dbversion:` i `dbname:` przy każdym zrzucie mówią, z jakiej wersji serwera pochodzi i jakie bazy zawiera. Kompletny plik kończy się linią `-- PostgreSQL database cluster dump complete` albo `-- Dump completed`.

Zaimportuj go do kontenera w tej samej lub nowszej wersji (PostgreSQL) albo w tej samej wersji głównej (MySQL i MariaDB), uruchomionego raz z pustym folderem danych, żeby się zainicjował. Host nie potrzebuje klienta bazy, kontener go ma:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Po jedną bazę z pełnego zrzutu: MySQL i MariaDB przyjmują `--one-database <name>` w poleceniu klienta. Zrzut PostgreSQL ma jedną sekcję na bazę, każda zaczyna się linią `\connect <name>`: skopiuj tę sekcję do osobnego pliku i zaimportuj go z `-d <name>` po utworzeniu bazy.

!!! warning "Zrzut zrobiony jako root niesie ze sobą użytkowników serwera"
    Pełny zrzut MySQL-a lub MariaDB zrobiony jako root zawiera bazę systemową `mysql`, więc jego import zastępuje konta nowego serwera, łącznie z hasłem roota, kontami ze zrzutu. W PostgreSQL komunikat `role ... already exists` o użytkowniku utworzonym przez kontener jest spodziewany i nieszkodliwy.
