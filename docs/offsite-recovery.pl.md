# Kopie poza siedzibą i odzyskiwanie

Kopie lokalne chronią Cię przed utraconym kontenerem lub złą aktualizacją. Replikacja poza siedzibą i przetestowany zestaw odzyskiwania chronią Cię przed całą maszyną, ransomware lub pożarem. Ta strona omawia replikację poza siedzibą, uczynienie tej kopii odporną na manipulacje, dowodzenie, że możesz przywracać, oraz odzyskiwanie, gdy sam BombVault zniknął.

## Replikacja poza siedzibą

Zachowaj szybką kopię lokalną i kopiuj ją do jednego lub więcej innych miejsc. Miejsca, do których kopiowana jest domena, wybierasz na karcie **Domeny** w **Ustawienia, Magazyn**, po jednym znaczniku na miejsce (zobacz [Miejsca przechowywania](storage-places.md#domains)). BombVault kopiuje tam nowe migawki poleceniem `restic copy` w trybie best-effort, więc nieudana kopia nigdy nie powoduje niepowodzenia kopii lokalnej. Miejsce, w którym domena jest przechowywana, nie musi być lokalne; zobacz [Domena przechowywana w miejscu zdalnym](#remote-primary-repositories).

- **Kilka miejsc kopii na domenę.** Domenę można kopiować do kilku miejsc naraz, na przykład do rest-servera u znajomego i do zasobnika B2. Przechowywanie, klasa pamięci, append-only, limity i budżet wzrostu należą do miejsca, więc każda kopia podlega zasadom miejsca, w którym ląduje.
- **Harmonogram kopiowania per domena** (edytowany obok każdego innego harmonogramu w Ustawienia, Harmonogramy): pozostaw pusty, aby kopiować po każdej kopii lokalnej, lub ustaw kadencję (na przykład `weekly Sun 03:00`), aby kopiować rzadziej, niż tworzysz kopie. **Kopiuj teraz** w wierszu domeny uruchamia kopiowanie na żądanie.
- **Przechowywanie per miejsce.** Każde miejsce ma własne zasady, więc miejsce poza siedzibą może trzymać kopie dłużej jako archiwum. Miejsce ze wszystkimi regułami na zero nigdy niczego nie przycina.
- **Limity przepustowości** per miejsce ograniczają tempo wysyłania i pobierania restic, aby kopiowanie nie nasycało Twojego łącza WAN.
- **Wskaźnik replikacji** pokazuje, która domena jest kopiowana w trakcie działania (na jej stronie i na panelu). To wskaźnik aktywności, a nie pasek procentowy, ponieważ `restic copy` nie udostępnia postępu czytelnego maszynowo.

!!! note "Przywróć z dowolnego miejsca"
    Każdy kontener, VM, zestaw plików, flash i konfiguracja aplikacji wylistowują swoje kopie jako jedną oś czasu obejmującą wszystkie miejsca, w których leży kopia. Kopia skopiowana do B2 pojawia się raz, oznaczona każdym miejscem, które ją przechowuje. Przywracanie sięga po pierwsze osiągalne miejsce, zaczynając od repozytorium, do którego element jest zapisywany, a dla każdego wiersza możesz wybrać inne miejsce. Miejsca poza siedzibą są odczytywane dopiero, gdy je otworzysz. Usunięcie w jednym miejscu najpierw sprawdza pozostałe i mówi, czy była to ostatnia kopia.

## Rozmieszczenie per element {#placement}

Każda karta kontenera, VM i zestawu plików ma wiersz **Rozmieszczenie** z trzema segmentami:

- **Lokalne** zapisuje element w repozytorium pokazanym pod **Zapisane na** i nigdzie go nie kopiuje. Użyj tego dla danych, które mają już drugą kopię, na przykład udziału, który leży na NAS.
- **Lokalne + poza siedzibą** zapisuje go tam tak samo i kopiuje do celów zaznaczonych pod **Kopiuj do**, po jednym znaczniku na każdy cel poza siedzibą domeny. Odznacz znacznik, a ten cel nie dostanie od tego elementu nic nowego.
- **Tylko poza siedzibą** zapisuje element wprost w miejscu pod **Wyślij do**, dowolnym innym niż to, w którym przechowywana jest domena. Gdy domena jest już kopiowana do tego miejsca, element dostaje repozytorium bezpośrednie obok kopii; w przeciwnym razie BombVault tworzy tam repozytorium dla domeny.

Lokalizacja jest ustalona od pierwszej kopii elementu, ponieważ BombVault nigdy nie przenosi kopii między repozytoriami. Kopie można zmieniać w dowolnym momencie. Cel, który przestaje otrzymywać element, zachowuje posiadane kopie i przycina je do własnego przechowywania przy następnym uruchomieniu poza siedzibą tej domeny; **Usuń w B2** na karcie usuwa je od razu. Gdy część tych kopii nie istnieje nigdzie indziej, potwierdzenie wylistowuje je według daty i prosi o nazwę elementu. Z celów tylko do dopisywania nie da się usuwać.

Pod wierszem karta mówi, dokąd trafia element i co faktycznie tam jest: w ilu lokalizacjach się znajduje, kiedy każdy cel widziano ostatnio oraz czy spełnione jest 3-2-1. Lokalizacją jest serwer z danymi oryginalnymi oraz każde miejsce w innej lokalizacji (zobacz [Poza obiektem](#off-the-premises-mark)). BombVault sprawdza kopie i lokalizacje; nie sprawdza części „dwa nośniki” zasady 3-2-1.

### Wartości domyślne domeny

Karta **Domeny** w Ustawienia, Magazyn ma jeden wiersz na domenę. **Kopiowane do** obowiązuje od razu dla każdego elementu bez własnego wyboru oraz dla folderów projektu stosów Compose. Gdy domena ma już kopie zapasowe, **Przechowywane w** obowiązuje dla nowego elementu od jego pierwszej kopii, a zmiana tego ustawienia nie przenosi żadnych kopii. Przed zapisem wiersz wymienia każde miejsce, które zyskuje lub traci elementy, i ile to znaczy migawek, a pytanie zawiera przełącznik **Zastosuj do elementów bez kopii zapasowych**, który dodatkowo ustawia nową wartość domyślną każdemu elementowi, który nie ma jeszcze kopii. **Wyjątki** wymieniają elementy z własnym wyborem.

Zaznaczenie nowego miejsca pod **Kopiowane do** sprawia, że otrzymuje ono każdy element, który nie jest ustawiony na Lokalne. Potwierdzenie mówi, ile to elementów i, gdzie to wiadome, ile historii to oznacza.

### Repozytoria bezpośrednie

Wybranie pod Tylko poza siedzibą miejsca, do którego domena jest już kopiowana, raz pyta o zgodę, a potem tworzy repozytorium bezpośrednie obok kopii, na przykład `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, i kieruje na nie element. Dla celu kopii bez miejsca wybór otwiera okno dialogowe z proponowanym adresem i testem połączenia, który niczego nie tworzy, a **Utwórz i użyj** tworzy repozytorium. Repozytorium bezpośrednie przejmuje klucz miejsca, klasę pamięci, limity, ustawienie append-only i przechowywanie, i zmienia się razem z nimi. Gdy nowy klucz miejsca nie może go otworzyć, repozytorium bezpośrednie zachowuje klucz, który ma, a zapis o tym informuje. Jego migawki niosą tag `bv:direct`, a każde inne przejście przechowywania je zachowuje, więc repozytorium bezpośrednie, które straciło powiązanie ze swoim miejscem, nigdy nie starzeje się według reguł lokalnych. Klucz B2 ograniczony do jednego folderu musi obejmować adres miejsca, a nie tylko folder domeny, inaczej folder obok pozostaje poza zasięgiem.

### Poza obiektem {#off-the-premises-mark}

Kopia liczy się jako osobna lokalizacja tylko wtedy, gdy jej miejsce znajduje się w innej lokalizacji. Miejsce w chmurze liczy się zawsze, a folder na tym serwerze Unraid nigdy; dla NAS, rest-servera lub serwera SFTP odpowiedz w szczegółach miejsca na pytanie **Gdzie stoi urządzenie?**, wybierając **Tutaj, w tym budynku** albo **W innej lokalizacji**. Odpowiedź liczy się tylko do lokalizacji i 3-2-1 na kartach i na panelu. Nie zmienia żadnej kopii.

### Po odbudowie

Wybory kopiowania żyją we własnych ustawieniach BombVault. Po odbudowie przez Odkryj bez przywróconego `/config` znikają, a skopiowanie wszystkiego wysłałoby ponownie do B2 elementy, które pominąłeś. Replikacja poza siedzibą każdej odbudowanej domeny dlatego wstrzymuje się. Panel pokazuje to na bursztynowo, a wiersz domeny na karcie Domeny oferuje **Potwierdź domyślne** z podglądem tego, co skopiuje następne uruchomienie, oraz nazwami w kopiach, które nie mają wpisu, a które możesz tam pominąć. Tylko potwierdzenie kończy wstrzymanie; import pliku ustawień przywraca reguły i wartości domyślne, ale go nie kończy.

## Domena przechowywana w miejscu zdalnym {#remote-primary-repositories}

Domena nie musi być przechowywana lokalnie. Dopóki w lokalizacji kopii zapasowych domeny nie ma żadnych kopii, wybierz miejsce zdalne pod **Przechowywane w** na karcie Domeny, a domena będzie tworzyć kopie zapasowe prosto tam, bez kopii lokalnej i bez kroku kopiowania. Repozytorium zdalne jest wtedy jedyną kopią, chyba że domena jest też kopiowana do innego miejsca. Każde miejsce zdalne ma te same zabezpieczenia:

- **Test połączenia**, zanim cokolwiek zostanie zapisane.
- **Limity przepustowości** dla samej kopii zapasowej: te same flagi `--limit-upload` i `--limit-download`, których używa kopiowanie.
- **Ochrona append-only**, sprawdzana tym samym aktywnym tamper testem. Gdy jest włączona, BombVault nigdy nie przycina repozytorium, ponieważ poświadczenia na tej maszynie nie mogą być w stanie usunąć jedynej kopii zapasowej.
- **Budżet wzrostu**, wyliczany z tego samego trendu rozmiaru, który śledzi karta Magazyn.

Domena przechowywana w miejscu zdalnym jest źródłem swoich kopii tak samo jak lokalna; zobacz [Kopie między miejscami z różnymi poświadczeniami](storage-places.md#different-credentials).

!!! note "Poświadczenia należą do miejsca"
    Miejsce zdalne ma własne poświadczenia. Miejsce skonfigurowane ze wspólnymi poświadczeniami chmury używa ich dalej, dopóki jego dostęp nie zostanie zmieniony w szczegółach miejsca.

## Niezmienna (append-only) kopia poza siedzibą

Oznacz repozytorium poza siedzibą jako append-only, aby ransomware lub skompromitowany host nie mogły usunąć ani nadpisać Twoich kopii. Druga strona (`restic/rest-server` działający w trybie `--append-only`) **wymusza** to. BombVault jedynie to **weryfikuje** i nigdy nie pokazuje zielonego na podstawie samej deklaracji konfiguracji.

Okno **Dodaj miejsce** zawiera gotowy do wklejenia przepis na rest-server w trybie append-only, z jednym użytkownikiem dla tego BombVault. W miejscu typu rest-server z włączonym **Append-only** przycisk **Sprawdź append-only** w szczegółach miejsca uruchamia tamper test dla każdej ścieżki domeny, każdej włączonej kopii i każdego repozytorium w tym miejscu i daje jedną odpowiedź dla całego miejsca, więc kopia poza siedzibą w trybie append-only jest osiągalna bez ręcznej edycji konfiguracji.

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

W miejscu **Sprawdź append-only** bada każdą ścieżkę domeny, każdą włączoną kopię i każde repozytorium z ich własnymi poświadczeniami i łączy werdykty w jedną odpowiedź: jeśli choć jedno repozytorium przyjmie usunięcie, całe miejsce dostaje *usuwanie dozwolone*.

## Próby DR

BombVault oferuje dwa poziomy dowodu, że Twoje kopie są faktycznie przywracalne, a nie tylko obecne.

- **Próby weryfikacji przywracalności (lokalne).** BombVault okresowo uruchamia `restic check --read-data-subset` (ograniczone, nigdy zapełniające dysk pełne przywracanie) i pokazuje odznakę *ostatnio zweryfikowano przywracalność* per domena. Kadencja znajduje się w Ustawienia, Harmonogramy; odznaka w Ustawienia, Integralność.
- **Próby DR (poza siedzibą).** BombVault przywraca prawdziwy cel z repozytorium poza siedzibą do jednorazowej piaskownicy, weryfikuje go plik po pliku i bajt po bajcie, a następnie sprząta. To dowodzi, że możesz odzyskać z kopii poza siedzibą, a nie tylko że repozytorium odpowiada. Próby obejmują tylko miejsca w innej lokalizacji, bo kopia w tym samym budynku nie dowodzi niczego na wypadek utraty budynku. Domena kopiowana do kilku takich miejsc przechodzi próbę w jednym z nich przy każdym zaplanowanym uruchomieniu, po kolei, a panel podaje miejsce ostatniej próby.

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

**1. Na VAULT postaw serwer append-only.** W BombVault na TOWER otwórz *Ustawienia → Magazyn*, kliknij **Dodaj miejsce**, wybierz **rest-server** i kliknij **Pokaż przepis**. Skopiuj blok **Szablon Unraid**, zapisz go na VAULT jako `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, następnie *Docker → Add Container* i wybierz **rest-server** z listy szablonów. Przed uruchomieniem wpisz pokazaną linię `htpasswd` na VAULT do `/mnt/user/appdata/rest-server/.htpasswd`. Hasło jest pokazywane raz i nigdy nie jest zapisywane; przepis wpisał już je i użytkownika do formularza na TOWER, więc nie zamykaj tego okna. Linia `htpasswd` niesie to samo hasło, już zahaszowane bcryptem, więc sam nie musisz niczego haszować.

    Zostaw `--append-only` w polu OPTIONS. Bez tego VAULT jest znów zwykłym udziałem.

**2. Na TOWER dodaj miejsce.** Wpisz adres VAULT, `http://VAULT:8000`, obok użytkownika i hasła, które wypełnił przepis, a potem kliknij **Testuj połączenie**. BombVault buduje z nich adres:

    rest:http://VAULT:8000/tower

Pierwszy segment ścieżki to użytkownik htpasswd, tutaj `tower`, a każda domena dostaje pod nim swój folder, na przykład `rest:http://VAULT:8000/tower/container`. Na pytanie **Gdzie stoi urządzenie?** odpowiedz **W innej lokalizacji**, kliknij **Dodaj** i zaznacz miejsce pod **Kopiowane do** dla domen, które mają tam trafiać.

**3. Na TOWER włącz Append-only** w sekcji **Ochrona** w szczegółach miejsca, a potem kliknij **Sprawdź append-only**. Test bada każdą ścieżkę domeny, każdą kopię i każde repozytorium w tym miejscu i daje jedną odpowiedź dla całego miejsca, która musi brzmieć *usuwanie odrzucane*. Co znaczą odpowiedzi:

| Wynik | Co się stało |
| --- | --- |
| **usuwanie odrzucane** | VAULT odmówił usunięcia. To jedyny stan zaliczony. |
| **usuwanie dozwolone** | VAULT przyjął usunięcie. Brakuje `--append-only` albo je usunięto. |
| komunikat zamiast wyniku | Test nie mógł się wykonać. Zwykle adres nie jest tym, którego używa sam restic, albo zmieniły się dane logowania. Nic nie jest zapisywane i nie uruchamia się alarm. |

**4. Na VAULT patrz, co przychodzi.** Włącz *Ustawienia → Odbiornik*, otwórz zakładkę **Odbiornik** i zarejestruj repozytorium tylko do odczytu.

!!! warning "Lokalizacja to ścieżka **wewnątrz** kontenera, zapisana względem montowania hosta"
    Wpisz `user/appdata/rest-server/tower/container`, a **nie** `/mnt/user/appdata/…`. BombVault działa w kontenerze, w którym `/mnt` hosta jest zamontowane gdzie indziej; bezwzględna ścieżka hosta tam nie istnieje. Jeśli ją wkleisz, BombVault poda ci ścieżkę względną, której należy użyć.

    **Wysyłający APP_KEY** to klucz TOWER, a nie VAULT. Znajdziesz go na TOWER w *Ustawienia → System*.

**5. Jeśli chcesz, zrób to wzajemnie.** Powtórz te same pięć kroków w drugą stronę: rest-server na TOWER przyjmujący kopię z VAULT. Wtedy każda maszyna wymusza niezmienność dla drugiej i żadna nie może usunąć kopii tej drugiej.

## Odzyskiwanie z przewodnikiem

Dedykowana zakładka **Odzyskiwanie** prowadzi świeżą lub odbudowaną instalację przez przypadek awarii, w jednym miejscu:

1. **Sprawdza, czy BombVault może odczytać Twoje kopie** (pułapka klucza szyfrowania od razu na wstępie).
2. **Przywraca własne ustawienia BombVault**, więc ścieżki kopii, cele poza siedzibą i poświadczenia, których potrzebuje reszta przepływu, są wstępnie wypełnione. Kopia ustawień jest odczytywana z miejsca, które wiersz Autokopia podaje w **Przechowywane w**, albo z kopii Autokopii w **Kopiowane do**, a krok pokazuje to miejsce z jego adresem. Aby czytać z innego miejsca, najpierw zmień wiersz Autokopia w kroku 3. Przywracanie jest stosowane przez samodzielny restart przez gniazdo Docker, więc działająca baza ustawień nigdy nie jest nadpisywana pod otwartym uchwytem.
3. **Podłącza Twoje istniejące kopie** przez wiersze karty Domeny: w wierszu każdej domeny wybierasz w **Przechowywane w** miejsce, w którym leżą jej kopie zapasowe, a w **Kopiowane do** miejsca z jej kopiami. Miejsce, którego nie oferuje jeszcze żaden wiersz, na przykład udział sieciowy, serwer albo zasobnik w chmurze, podłączasz przez **Dodaj miejsce**, to samo okno co w Ustawienia, Magazyn. **Połącz i wyświetl podgląd** sprawdza potem, czy kopie da się odczytać.
4. **Odkrywa** kontenery, VM, zestawy plików i zbiory danych ZFS w nim przechowywane.
5. **Przywraca kontenery i VM za jednym razem** (pozostawiając zatrzymanymi, więc uruchamiasz je świadomie) i wypisuje zestawy plików oraz elementy ZFS do przywrócenia po kolei; elementy ZFS wracają wyłączone. Twój zestaw odzyskiwania jest o jedno kliknięcie stąd.

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

!!! warning "Najnowsza migawka nie zawsze jest tą do przywrócenia"
    Od restic 0.17 polecenie `restic snapshots` pokazuje rozmiar każdej migawki. Po utracie danych najnowsza migawka może być tą opróżnioną, więc nie przywracaj migawki dużo mniejszej niż poprzednie. Po ataku ransomware może to być migawka zaszyfrowana o zwykłym rozmiarze. Jeśli BombVault nadal działa, najpierw zajrzyj na jego stronę **Anomalie**: wskazuje ostatnią dobrą kopię. Przywracanie nie potrzebuje żadnych danych o anomaliach z BombVault, a wstrzymanie retencji zawsze tylko zachowuje więcej migawek.

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

## Odzyskiwanie zrzutu bazy danych {#database-dumps}

Zrzut bazy danych jest osobnym punktem przywracania w repozytorium kontenerów, z etykietą `dbdump:<container>` i jednym plikiem, `/dbdump/<container>.sql`. BombVault wypisuje je, pobiera i importuje w sekcji **Kopie**; poniżej te same kroki z samym resticiem, na dzień, w którym BombVaulta nie ma.

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
