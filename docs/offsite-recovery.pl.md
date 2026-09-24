# Kopie poza siedzibą i odzyskiwanie

Kopie lokalne chronią Cię przed utraconym kontenerem lub złą aktualizacją. Replikacja poza siedzibą i przetestowany zestaw odzyskiwania chronią Cię przed całą maszyną, ransomware lub pożarem. Ta strona omawia replikację poza siedzibą, uczynienie tej kopii odporną na manipulacje, dowodzenie, że możesz przywracać, oraz odzyskiwanie, gdy sam BombVault zniknął.

## Replikacja poza siedzibą

Zachowaj szybką kopię lokalną i dodaj jedną lub więcej replik poza siedzibą. Ustaw repozytorium per domena w zakładce **Ustawienia, Poza siedzibą**. BombVault replikuje tam nowe migawki poleceniem `restic copy` w trybie best-effort, więc potknięcie poza siedzibą nigdy nie powoduje niepowodzenia kopii lokalnej. Repozytorium lokalne pozostaje główne.

- **Wiele celów poza siedzibą na domenę.** Każda domena (kontenery, VM, flash, config i zestawy plików) może replikować do kilku celów poza siedzibą naraz, nie tylko jednego, więc możesz utrzymywać na przykład rest-server na maszynie znajomego oraz bucket S3 równolegle. Dodaj dodatkowe cele w Ustawienia, Poza siedzibą, każdy z własnym repozytorium, klasą pamięci S3, flagą append-only, przechowywaniem i budżetem wzrostu. Istniejąca pojedyncza konfiguracja poza siedzibą jest przenoszona nietknięta jako pierwszy cel, a każdy cel domeny replikuje zgodnie z harmonogramem poza siedzibą tej domeny.
- **Harmonogram poza siedzibą per domena** (edytowany obok każdego innego harmonogramu w Ustawienia, Harmonogramy): pozostaw pusty, aby replikować po każdej kopii lokalnej, lub ustaw kadencję (na przykład `weekly Sun 03:00`), aby wysyłać poza siedzibę rzadziej, niż tworzysz kopie lokalne. Przycisk **Replikuj teraz** obsługuje uruchomienia na żądanie.
- **Przechowywanie poza siedzibą** znajduje się w Ustawienia, Poza siedzibą, więc możesz trzymać kopie poza siedzibą dłużej jako archiwum. Pozostaw politykę całą na zero, aby nigdy nie przycinać automatycznie migawek poza siedzibą.
- **Limity przepustowości** (Ustawienia, Poza siedzibą) ograniczają tempo wysyłania/pobierania restic, aby replikacja nie nasycała Twojego łącza WAN.
- **Wskaźnik replikacji** pokazuje, która domena jest replikowana w trakcie działania (na jej stronie i na panelu). To wskaźnik aktywności, a nie pasek procentowy, ponieważ `restic copy` nie udostępnia postępu czytelnego maszynowo.

!!! note "Przywróć z dowolnego miejsca"
    Każdy kontener, VM, zestaw plików, flash i konfiguracja aplikacji wylistowują swoje kopie jako jedną oś czasu obejmującą wszystkie miejsca, w których leży kopia. Kopia skopiowana do B2 pojawia się raz, oznaczona każdym miejscem, które ją przechowuje. Przywracanie sięga po pierwsze osiągalne miejsce, zaczynając od repozytorium, do którego element jest zapisywany, a dla każdego wiersza możesz wybrać inne miejsce. Miejsca poza siedzibą są odczytywane dopiero, gdy je otworzysz. Usunięcie w jednym miejscu najpierw sprawdza pozostałe i mówi, czy była to ostatnia kopia.

## Rozmieszczenie per element {#placement}

Każda karta kontenera, VM i zestawu plików ma wiersz **Rozmieszczenie** z trzema segmentami:

- **Lokalne** zapisuje element w repozytorium pokazanym pod **Zapisane na** i nigdzie go nie kopiuje. Użyj tego dla danych, które mają już drugą kopię, na przykład udziału, który leży na NAS.
- **Lokalne + poza siedzibą** zapisuje go tam tak samo i kopiuje do celów zaznaczonych pod **Kopiuj do**, po jednym znaczniku na każdy cel poza siedzibą domeny. Odznacz znacznik, a ten cel nie dostanie od tego elementu nic nowego.
- **Tylko poza siedzibą** zapisuje element wprost w miejscu pod **Wyślij do**: repozytorium bezpośrednim obok celu poza siedzibą albo zdalnym repozytorium skonfigurowanym w Ustawienia, Ścieżki i pamięć, Repozytoria.

Lokalizacja jest ustalona od pierwszej kopii elementu, ponieważ BombVault nigdy nie przenosi kopii między repozytoriami. Kopie można zmieniać w dowolnym momencie. Cel, który przestaje otrzymywać element, zachowuje posiadane kopie i przycina je do własnego przechowywania przy następnym uruchomieniu poza siedzibą tej domeny; **Usuń w B2** na karcie usuwa je od razu. Gdy część tych kopii nie istnieje nigdzie indziej, potwierdzenie wylistowuje je według daty i prosi o nazwę elementu. Z celów tylko do dopisywania nie da się usuwać.

Pod wierszem karta mówi, dokąd trafia element i co faktycznie tam jest: ile miejsc go przechowuje, kiedy każdy cel widziano ostatnio oraz czy spełnione jest 3-2-1. Miejscem jest serwer z danymi oryginalnymi, każdy cel poza siedzibą i każde repozytorium oznaczone **Poza obiektem**. BombVault sprawdza kopie i miejsca; nie sprawdza części „dwa nośniki” zasady 3-2-1.

### Domyślne rozmieszczenie

Ustawienia, Ścieżki i pamięć, **Domyślne rozmieszczenie** ma jeden wiersz na domenę z tymi samymi trzema segmentami. Kopie stosują się od razu do każdego elementu bez własnego wyboru oraz do folderów projektu stosów Compose. Lokalizacja stosuje się do nowego elementu przy jego pierwszej kopii; jej zmiana nie przenosi żadnych kopii. Przed zapisem wiersz wymienia każdy cel, który zyskuje lub traci elementy, i ile to znaczy migawek. **Zastosuj do elementów bez kopii zapasowych** przywraca do domyślnego każdy element, który nie ma jeszcze kopii.

Nowy cel poza siedzibą otrzymuje każdy element, który nie jest ustawiony na Lokalne. Okno dialogowe, które go dodaje, mówi, ile to elementów i, gdzie to wiadome, ile historii to oznacza, oraz proponuje pominięcie elementów już wykluczonych z innych celów.

### Repozytoria bezpośrednie

Wybranie repozytorium bezpośredniego celu pod Tylko poza siedzibą otwiera okno dialogowe z proponowaną lokalizacją obok celu, na przykład `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, oraz testem połączenia, który niczego nie tworzy. **Utwórz i użyj** tworzy repozytorium i kieruje na nie element. Repozytorium bezpośrednie przejmuje klucz celu, klasę pamięci, limity, ustawienie append-only i przechowywanie, i zmienia się razem z nimi; karta Repozytoria pokazuje je jako tylko do odczytu. Gdy nowy klucz celu nie może go otworzyć, repozytorium bezpośrednie zachowuje klucz, który ma, a zapis o tym informuje. Jego migawki niosą tag `bv:direct`, a każde inne przejście przechowywania je zachowuje, więc repozytorium bezpośrednie, które straciło łączność ze swoim celem, nigdy nie starzeje się według reguł lokalnych. Do B2 uzyskuje się dostęp przez jego punkt końcowy S3, podając identyfikator klucza i klucz aplikacji jako dane uwierzytelniające S3; klucz ograniczony do własnego katalogu celu nie sięga do katalogu obok niego, więc ogranicz klucz zamiast tego do katalogu nad celem.

### Poza obiektem

Nazwane repozytorium można oznaczyć jako **Poza obiektem** na karcie Repozytoria. Repozytoria zdalne zaczynają oznaczone; wyłącz to dla rest-servera w tym samym budynku. Oznaczenie liczy się tylko do miejsc i 3-2-1 na kartach. Nie zmienia żadnej kopii.

### Po odbudowie

Wybory kopiowania żyją we własnych ustawieniach BombVault. Po odbudowie przez Odkryj bez przywróconego `/config` znikają, a skopiowanie wszystkiego wysłałoby ponownie do B2 elementy, które pominąłeś. Replikacja poza siedzibą każdej odbudowanej domeny dlatego wstrzymuje się. Panel pokazuje to na bursztynowo, a Domyślne rozmieszczenie oferuje **Potwierdź domyślne** z podglądem tego, co skopiuje następne uruchomienie, oraz nazwami w kopiach, które nie mają wpisu, a które możesz tam pominąć. Tylko potwierdzenie kończy wstrzymanie; import pliku ustawień przywraca reguły i wartości domyślne, ale go nie kończy.

## Zdalne repozytoria podstawowe {#remote-primary-repositories}

Ścieżka kopii domeny (Ustawienia, Ścieżki i magazyn) nie ogranicza się do lokalnego katalogu: skieruj ją wprost na zdalne repozytorium restica (`s3:...`, `rest:http://host:8000/repo`, `sftp:użytkownik@host:/repo`, `rclone:remote:bucket/ścieżka`), a BombVault będzie archiwizował prosto tam, bez osobnej kopii lokalnej i bez kroku replikacji. To naprawdę inny układ niż replikacja poza siedzibę powyżej: tam repozytorium lokalne jest podstawowe, a zewnętrzne jest jego archiwum w miarę możliwości; tutaj repozytorium zdalne **jest** podstawowe i jest jedyną kopią, dopóki nie skonfigurujesz dla tej domeny również replikacji poza siedzibę (albo drugiego repozytorium zdalnego).

Każde z pięciu pól ścieżki (Kontenery, Maszyny wirtualne, Flash, Konfiguracja, Pliki) ma tuż obok przełącznik **Lokalne / Zdalne**:

- **Lokalne** pokazuje znaną przeglądarkę katalogów.
- **Zdalne** zamienia ją na zwykłe pole URL oraz przycisk otwierający to samo okno testu połączenia i danych logowania, którego używają cele poza siedzibą, tyle że skonfigurowane dla tego repozytorium podstawowego. Dostajesz stamtąd:
    - **Test połączenia** z rzeczywistą ścieżką, zanim na niej polegniesz.
    - **Ograniczenia pasma** (wysyłanie i pobieranie), żeby zaplanowana kopia do zdalnego repozytorium podstawowego nie zapchała łącza WAN: te same przełączniki restica `--limit-upload` i `--limit-download`, których używa replikacja poza siedzibę, zastosowane do samej kopii.
    - **Ochronę append-only (niezmienność)**, sprawdzaną tym samym aktywnym testem manipulacji (prawdziwa próba DELETE po drugiej stronie), który dostają cele poza siedzibą. Gdy jest włączona, BombVault odmawia przycinania repozytorium: skoro nie stoi za nim osobna kopia lokalna, dane logowania na tej maszynie nie mogą być w stanie usunąć jedynej kopii zapasowej.
    - **Alarm budżetu wzrostu**, wyliczany z tego samego trendu rozmiaru repozytorium, który karta Magazyn i tak już śledzi.

Nic z tego nie jest obowiązkowe: wpisana ręcznie ścieżka zdalna bez zapisanych ustawień bezpieczeństwa archiwizuje dokładnie tak jak dotąd (nieograniczone pasmo, możliwe przycinanie, brak alarmu budżetu). Okno bezpieczeństwa jest na wypadek, gdy chcesz te same zabezpieczenia, jakie dostaje kopia poza siedzibą, bez zakładania osobnego celu zewnętrznego tylko po to.

!!! note "Dane logowania do chmury i REST są wspólne"
    Zdalne repozytorium podstawowe uwierzytelnia się tymi samymi danymi S3/REST, które są ustawione w Ustawienia, Poza siedzibą, Dane logowania do chmury. Osobnego magazynu danych logowania dla repozytoriów podstawowych nie ma.

## Niezmienna (append-only) kopia poza siedzibą

Oznacz repozytorium poza siedzibą jako append-only, aby ransomware lub skompromitowany host nie mogły usunąć ani nadpisać Twoich kopii. Druga strona (`restic/rest-server` działający w trybie `--append-only`) **wymusza** to. BombVault jedynie to **weryfikuje** i nigdy nie pokazuje zielonego na podstawie samej deklaracji konfiguracji.

Kreator **konfiguracji poza siedzibą z przewodnikiem** prowadzi Cię od wyboru backendu (rest-server / rclone / S3) przez gotowy do wklejenia fragment wdrożenia rest-server, test połączenia, przełącznik niezmienności (który natychmiast uruchamia tamper test) i strategię przechowywania, więc kopia poza siedzibą w trybie append-only jest osiągalna bez ręcznej edycji konfiguracji.

!!! note "Udane usunięcie w `/locks/` jest oczekiwane"
    Append-only nie oznacza, że nic już nie da się usunąć. restic musi zakładać i zwalniać własne blokady, więc `/locks/` celowo pozostaje zapisywalny i usuwalny. Migawki i stojące za nimi dane, czyli dokładnie to, na co celowałoby ransomware, nie dają się usunąć. Jeśli sam sprawdzisz zdalną stronę, udane usunięcie w `/locks/` jest poprawnym zachowaniem, a nie luką.

!!! warning "Niezmienne repozytoria nigdy nie są przycinane z tej maszyny"
    Niezmienna kopia poza siedzibą celowo nigdy nie przycina starych migawek. Ustaw dla niej **alarm budżetu wzrostu**, aby otrzymać alert, zanim rozmiar repozytorium wymknie się spod kontroli.

## Tamper test

BombVault okresowo dowodzi gwarancji append-only, faktycznie próbując usunięcia względem repozytorium poza siedzibą, skierowanego na nieistniejący obiekt:

- **Odmowa** oznacza ochronę.
- **Przyjęcie** oznacza brak ochrony.
- Wynik **niejednoznaczny** (serwer nieosiągalny, błąd uwierzytelniania) nigdy nie odwraca zapisanego werdyktu.

Prawdziwe przejście z chronionego do niechronionego wyzwala pojedynczy alert.

## Próby DR

BombVault oferuje dwa poziomy dowodu, że Twoje kopie są faktycznie przywracalne, a nie tylko obecne.

- **Próby weryfikacji przywracalności (lokalne).** BombVault okresowo uruchamia `restic check --read-data-subset` (ograniczone, nigdy zapełniające dysk pełne przywracanie) i pokazuje odznakę *ostatnio zweryfikowano przywracalność* per domena. Kadencja znajduje się w Ustawienia, Harmonogramy; odznaka w Ustawienia, Integralność.
- **Próby DR (poza siedzibą).** BombVault przywraca prawdziwy cel z repozytorium poza siedzibą do jednorazowej piaskownicy, weryfikuje go plik po pliku i bajt po bajcie, a następnie sprząta. To dowodzi, że możesz odzyskać z kopii poza siedzibą, a nie tylko że repozytorium odpowiada.

**Karta wyników ochrony przed ransomware** na panelu zbiera to w postawę zielony / bursztynowy / czerwony per domena, z listą kontrolną ze znacznikiem wieku (skonfigurowano poza siedzibą, zweryfikowano append-only, replikacja aktualna, próba przywracania zaliczona, szyfrowanie włączone, ustawiono strategię przycinania). Każdy czerwony wiersz linkuje bezpośrednio do naprawy, a karta przechodzi na zielony tylko na podstawie zweryfikowanych faktów.

## Panel odbiorcy (strona odbierająca)

![Strona odbierająca, obserwowana tylko do odczytu, z kontrolą spójności na tej maszynie.](assets/screenshots/receiver.png)

*Strona odbierająca, obserwowana tylko do odczytu, z kontrolą spójności na tej maszynie.*

Wszystko powyżej to strona *wysyłająca*. Na maszynie, która **odbiera** niezmienne kopie poza siedzibą od innego BombVault, panel odbiorcy daje Ci niezależne, tylko do odczytu monitorowanie tych repozytoriów na sprzęcie odbierającym, więc ciche niepowodzenie po drugiej stronie nie pozostaje niezauważone.

Włącz przełącznik **Odbiorca** w Ustawieniach, aby odsłonić zakładkę **Odbiorca**. Jest domyślnie wyłączony; włącz go tylko na maszynie, która faktycznie odbiera niezmienne kopie poza siedzibą. Następnie zarejestruj otrzymane repozytorium (tylko do odczytu, otwarte kluczem instancji wysyłającej), aby uzyskać:

- **Inwentarz migawek pogrupowany według źródła**, więc widzisz dokładnie, które kontenery, VM i zestawy plików dotarły.
- **Ostatnio otrzymane** per źródło, więc wiesz, jak świeże jest każde z nich.
- **Niezależny `restic check`** uruchomiony na sprzęcie odbierającym, więc integralność jest weryfikowana tam, gdzie dane faktycznie się znajdują, a nie tylko po stronie nadawcy.
- **Wyłącznik bezpieczeństwa:** alert, gdy źródło przestaje wysyłać w ustawionym przez Ciebie oknie.
- **Alerty integralności:** alert, gdy kontrola po stronie odbierającej zawiedzie.

Odbiorca jest ściśle tylko do odczytu. Nigdy nie zapisuje do otrzymanego repozytorium, więc nigdy nie może naruszyć gwarancji append-only, na której polega nadawca.

## Pełny przykład: dwie maszyny Unraid, od początku do końca

Powyżej opisano części. To jest jedna kompletna konfiguracja z prawdziwymi wartościami, bo części łatwiej złożyć, gdy raz się je widziało złożone.

Dwie maszyny: **TOWER** uruchamia kontenery i wysyła kopie, **VAULT** je przyjmuje i wymusza niezmienność. Podstaw własne nazwy, adresy i ścieżki udziałów.

**1. Na VAULT postaw serwer append-only.** W BombVault na TOWER przejdź do *Ustawienia → Poza siedzibą → kreator*, wybierz **rest-server** i wygeneruj przepis. Skopiuj zakładkę **Szablon Unraid (XML)**, zapisz ją na VAULT jako `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, następnie *Docker → Add Container* i wybierz **rest-server** z listy szablonów. Przed uruchomieniem wpisz pokazaną linię `htpasswd` na VAULT do `/mnt/user/appdata/rest-server/.htpasswd`. Jednorazowe hasło pokazywane jest raz i nigdy nie jest zapisywane: skopiuj je teraz. Ta linia niesie to samo hasło, już zahaszowane bcryptem: jawny tekst trafia do danych REST na TOWER, zahaszowana linia do `.htpasswd` na VAULT. Sam nie musisz niczego haszować.

    Zostaw `--append-only` w polu OPTIONS. O to właśnie chodzi: bez tego VAULT znów jest zwykłym udziałem.

**2. Na TOWER skieruj repozytorium zdalne na niego.** Adres repozytorium ma wzorzec, który wypisuje przepis:

    rest:http://VAULT:8000/bombvault-containers/containers

Pierwszy segment ścieżki to użytkownik htpasswd, drugi to repozytorium. Wpisz wygenerowanego użytkownika i hasło jako dane REST celu, a potem uruchom **test połączenia**.

**3. Na TOWER włącz «Niezmienne».** Test naruszenia uruchamia się od razu i musi zgłosić *chronione*. Co znaczą odpowiedzi:

| Wynik | Co się stało |
| --- | --- |
| **chronione** | VAULT odmówił usunięcia. To jedyny stan zaliczony. |
| **NIE chronione** | VAULT przyjął usunięcie. Brakuje `--append-only` albo je usunięto. |
| **nierozstrzygnięte** | Ani jedno, ani drugie. Zwykle adres nie jest tym, którego używa sam restic, albo zmieniły się dane logowania. Nic nie jest zapisywane i nie uruchamia się alarm. |

**4. Na VAULT patrz, co przychodzi.** Włącz *Ustawienia → Odbiornik*, otwórz zakładkę **Odbiornik** i zarejestruj repozytorium tylko do odczytu.

!!! warning "Lokalizacja to ścieżka **wewnątrz** kontenera, zapisana względem montowania hosta"
    Wpisz `user/appdata/rest-server/bombvault-containers/containers`, a **nie** `/mnt/user/appdata/…`. BombVault działa w kontenerze, w którym `/mnt` hosta jest zamontowane gdzie indziej; bezwzględna ścieżka hosta tam nie istnieje. Jeśli ją wkleisz, BombVault poda ci teraz ścieżkę względną, której należy użyć.

    **Wysyłający APP_KEY** to klucz TOWER, a nie VAULT. Znajdziesz go na TOWER w *Ustawienia → System*.

**5. Jeśli chcesz, zrób to wzajemnie.** Powtórz te same pięć kroków w drugą stronę: rest-server na TOWER przyjmujący kopię z VAULT. Wtedy każda maszyna wymusza niezmienność dla drugiej i żadna nie może usunąć kopii tej drugiej.

## Odzyskiwanie z przewodnikiem

Dedykowana zakładka **Odzyskiwanie** prowadzi świeżą lub odbudowaną instalację przez przypadek awarii, w jednym miejscu:

1. **Najpierw przywraca własne ustawienia BombVault**, więc ścieżki kopii, cele poza siedzibą i poświadczenia, których potrzebuje reszta przepływu, są wstępnie wypełnione (stosowane przez samodzielny restart przez gniazdo Docker, więc działająca baza ustawień nigdy nie jest nadpisywana pod otwartym uchwytem).
2. **Sprawdza, czy BombVault może odczytać Twoje kopie** (pułapka klucza szyfrowania od razu na wstępie).
3. Pozwala Ci **wskazać istniejące repozytorium** (lokalne lub poza siedzibą).
4. **Odkrywa** kontenery, VM i zestawy plików w nim przechowywane.
5. **Przywraca je wszystkie** (pozostawiając zatrzymanymi, więc uruchamiasz je świadomie), z Twoim zestawem odzyskiwania o jedno kliknięcie stąd.

!!! note "Kopie poza siedzibą czekają po odbudowie"
    Gdy krok 4 odbudowuje wpisy bez starych ustawień, replikacja poza siedzibą tych domen wstrzymuje się, dopóki domyślne rozmieszczenie nie zostanie potwierdzone. Zobacz [Rozmieszczenie per element](#placement).

!!! tip "Zaplanowana migracja kontra awaria"
    Odzyskiwanie z przewodnikiem przywraca własne ustawienia BombVault z kopii zapasowej. Do *zaplanowanego* przejścia na nową maszynę możesz zamiast tego przenieść swoją konfigurację bezpośrednio za pomocą karty **Eksport i import ustawień** (przenośny plik JSON). Zobacz [Konfiguracja](configuration.md#portable-settings-export-and-import).

### Przywracanie z innego repozytorium BombVault

Osobna karta w zakładce **Odzyskiwanie** otwiera repozytorium *innej* instancji BombVault (udział zamontowany pod `/mnt` lub zdalny URL) za pomocą **`APP_KEY` tej instancji**, w jednorazowej sesji tylko do odczytu. Przeglądaj przechowywane tam kontenery, VM i zestawy plików, wybierz migawkę i przywróć ją, a przywrócony obiekt staje się normalnym lokalnym kontenerem, VM lub zestawem plików. Nic nigdy nie jest zapisywane do drugiego repozytorium, a Twoje własne ustawienia kopii pozostają nietknięte (sesja żyje w pamięci i wygasa sama). Przeniesienie kontenera z serwera A na serwer B nie oznacza już przekierowywania ustawień repozytorium i cofania ich potem. Federacja serwer-do-serwera na żywo jest wyraźnie poza zakresem; to celowe jednorazowe pobranie.

## Zestaw odzyskiwania klucza szyfrowania

To element, który umożliwia odzyskiwanie po awarii nawet wtedy, gdy nie ma działającego BombVault.

Jedno kliknięcie pobiera **klucz główny**, **wyprowadzone hasło restic** oraz **dokładne lokalizacje repozytoriów i polecenia**, więc możesz przywracać wprost za pomocą CLI restic na dowolnej maszynie. Przypomnienie na panelu nęka Cię, dopóki go nie zapiszesz.

!!! danger "Przechowuj zestaw odzyskiwania poza serwerem"
    Zestaw zawiera sekret, który odszyfrowuje Twoje kopie. Trzymaj go w bezpiecznym miejscu, oddzielnie od serwera (menedżer haseł, wydrukowana kopia w sejfie). Jeśli stracisz zarówno BombVault, jak i `APP_KEY` bez zestawu odzyskiwania, Twoich zaszyfrowanych kopii nie da się odzyskać.

### Gdy zestawu nie ma pod ręką

Hasło nie jest nigdzie zapisane, jest **wyliczane** z `APP_KEY`. Mając klucz i powłokę, możesz je odtworzyć samodzielnie:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

To HMAC-SHA256 po stałym ciągu `bombvault:restic-repo`, z surowymi bajtami szesnastkowego `APP_KEY` jako kluczem, wypisany jako 64 małe znaki szesnastkowe. Ta sama wartość jest w zestawie jako wyprowadzone hasło restic; to tutaj jest na dzień, w którym zestaw leży gdzie indziej niż ty.

!!! warning "Dla otrzymanego repozytorium użyj klucza instancji WYSYŁAJĄCEJ"
    Repozytorium, które trafiło tu przez replikację poza siedzibę, utworzyła maszyna, która je wysłała, swoim **własnym** `APP_KEY`. Wyprowadzenie z klucza maszyny odbierającej daje hasło, które restic odrzuca, co wygląda dokładnie jak uszkodzone repozytorium, choć nim nie jest. To zwykły powód, dla którego `restic check` na otrzymanym repozytorium wciąż pyta o hasło.

Ponieważ definicje odzyskiwania znajdują się **wewnątrz** każdego repozytorium (`<repo>/def`, `<repo>/vm-def`), skopiowany folder repozytorium jest w pełni samowystarczalny, więc zestaw plus repozytorium to wszystko, czego potrzebuje przywracanie na goły metal.
