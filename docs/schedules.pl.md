# Harmonogramy

**Ustawienia, Harmonogram** zawierają każdy plan: pełną kopię, która obejmuje wszystko, własny plan dla sekcji, kopie poza siedzibą, to, które elementy biorą udział, i w jakiej kolejności. Pojedynczy element może dostać własny plan na swojej stronie. Ta strona wyjaśnia, jak to wszystko do siebie pasuje, i przechodzi przez typowe ustawienie.

## Plany czytane jak zdania {#plans}

Każdy plan to przycisk, który czyta się jak zdanie, na przykład *codziennie o 03:00* albo *co 2 dni o 01:30*. Kliknięcie otwiera go z pięcioma trybami:

| Tryb | Co ustawiasz |
|---|---|
| **Wyłączone** | Nic nie działa według tego planu. |
| **Codziennie** | **Godzina** |
| **Co tydzień** | **Dni** i **Godzina** |
| **Co N dni** | **Co** tyle **dni** i **Godzina**. Liczy od ostatniego zaplanowanego przebiegu, więc ręczna kopia jednego elementu nie przesuwa planu. |
| **Cron** | **Wyrażenie** z pięciu pól, na przykład `0 */6 * * *` dla co 6 godzin. **Przykłady** oferują kilka, a okno pokazuje najbliższe przebiegi. |

Wszystkie godziny są w strefie czasowej serwera. Na Unraid to strefa samego systemu. Gdzie indziej ustaw `TZ`, bo inaczej każdy plan działa w UTC, zobacz [Zmienne środowiskowe](configuration.md#environment-variables).

## Tydzień {#week}

Karta **Tydzień** rysuje każdy zaplanowany przebieg od poniedziałku do niedzieli, jedna linia na dzień: kopie, kopie poza siedzibą i kontrole. Wskaż kreskę, aby przeczytać, co tam startuje. Gdy trzy plany startują w ciągu dwóch godzin, to miejsce jest zaznaczone, żebyś mógł je rozsunąć.

![Ustawienia, Harmonogram: pasek Tydzień na górze, potem pełna kopia i plany według sekcji.](assets/screenshots/schedule/week-dark.png#only-dark)
![Ustawienia, Harmonogram: pasek Tydzień na górze, potem pełna kopia i plany według sekcji.](assets/screenshots/schedule/week-light.png#only-light)

## Pełna kopia {#full-backup}

**Pełna kopia** to domyślny plan dla wszystkiego. Ustawia ją **Kiedy działają kopie**: jeden przebieg, który kopiuje po kolei każdą sekcję, kontenery, maszyny wirtualne, flash, foldery, zbiory danych ZFS, a autokopię na końcu. Linia pod spodem mówi, które sekcje obejmuje. **Wyłączone** wyłącza ją, a wtedy działa tylko to, co ma własny plan.

- **Nadrabiaj pominięte kopie zapasowe po starcie**: gdy serwer był wyłączony w zaplanowanej porze, BombVault wykonuje tę kopię mniej więcej dwie minuty po starcie. Własny plan elementu i plan poza siedzibą czekają na swoją następną zaplanowaną porę, tak samo jak plan, który nigdy się nie powiódł.
- **Polecenia przed i po**: **Polecenie przed pełną kopią** działa na serwerze przez `sh -c`, zanim przebieg ruszy, na przykład skrypt zatrzymujący usługę. Jeśli się nie powiedzie, pełna kopia i tak się wykona. **Polecenie po pełnej kopii** działa raz, gdy każda sekcja została spróbowana, nawet jeśli któraś zawiodła, co pasuje do pingu potwierdzającego, że cały serwer jest chroniony. Polecenia dla pojedynczego kontenera są na jego stronie.

**Utwórz kopię wszystkiego teraz** na stronie Kopie zapasowe uruchamia ten sam przebieg ręcznie, dla każdej włączonej sekcji.

## Plany według sekcji {#plans-per-section}

Sekcja może opuścić pełną kopię i działać o własnej porze. W **Harmonogramy według sekcji** **Wyjątek** dodaje sekcję z własnym planem, a **Usuń wyjątek** odsyła ją z powrotem. Linia w karcie mówi, co robią pozostałe, na przykład **Wszystkie pozostałe sekcje działają w pełnej kopii.**

Sekcja z własnym planem nie jest kopiowana ponownie w pełnej kopii, więc nic nie jest kopiowane dwa razy. To samo dotyczy pojedynczego elementu z własnym planem.

**Kopie poza siedzibą według sekcji** działają tak samo dla kopiowania. Domyślnie każda sekcja kopiuje do swoich miejsc poza siedzibą zaraz po każdej kopii. **Wyjątek** daje jednej sekcji własny plan kopiowania, na przykład żeby maszyny wirtualne kopiowały do Backblaze B2 tylko w niedzielne noce. Karta **Kopiuj** samego miejsca docelowego też może ustawić **Kiedy kopiować**, zobacz [Kopie poza siedzibą](offsite-recovery.md#off-site-copies).

## Zaplanowane elementy {#scheduled-entries}

**Zaplanowane elementy** mówią dla każdej sekcji, ile elementów bierze udział, na przykład **38 z 41 zaplanowanych**. **Elementy** otwierają listę z przełącznikiem **W harmonogramie** przy każdym elemencie. Element, który jest wyłączony, jest kopiowany tylko wtedy, gdy uruchomisz go ręcznie. Liczy się jako niechroniony, chyba że oznaczysz go **Nie kopiuj**.

## Kolejność {#order}

**Kolejność** ustawia porządek, w jakim zaplanowane przebiegi i pełna kopia kopiują kontenery i maszyny wirtualne. Przeciągnij element na miejsce albo użyj strzałek, a każde przesunięcie zapisuje się od razu. Elementy, których nie uporządkujesz, są kopiowane od najbardziej spóźnionego. Pojedyncza ręczna kopia nie jest tym dotknięta.

**Ponowne uruchomienie po kopii zapasowej** w tej samej karcie dotyczy kontenerów, które **Zatrzymaj inne kontenery** wyłączyło podczas kopii. Startują z powrotem w kolejności zależności, a z **Zaczekaj, aż zależności będą sprawne, zanim uruchomisz następny** każdy musi zgłosić się jako sprawny lub działający, zanim wystartuje następny. **Limit czasu sprawności na kontener (sekundy)** ogranicza oczekiwanie, domyślnie 120, w zakresie od 5 do 3600.

## Migawka zamiast zatrzymania {#snapshot-instead-of-stop}

**Kontenery na ZFS zatrzymuj tylko na czas migawki** jest domyślnie włączone. Kontener, którego dane leżą na ZFS, jest zatrzymywany tylko na czas, gdy BombVault robi migawkę tych danych, zwykle kilka sekund, a kopia jest potem czytana z migawki. Potrzebuje połączenia SSH z hostem, z którego korzystają zbiory danych ZFS. Kontener z częścią danych gdzie indziej jest zatrzymany na całą kopię jak dawniej, a jego przebieg mówi dlaczego, natomiast kontener bez żadnych danych na ZFS jest zatrzymywany bez adnotacji. Działa na udziale użytkownika Unraid leżącym na jednej puli ZFS. Na TrueNAS aplikacja potrzebuje też uprawnienia `SYS_CHROOT`, zobacz [TrueNAS SCALE](getting-started.md#truenas). Pojedynczy kontener można ustawić inaczej na jego stronie.

Kopia ląduje pod tymi samymi ścieżkami i tagami co kopia zrobiona przy zatrzymanym kontenerze, więc przywracanie i kontrole traktują obie tak samo. Polecenie przed kopią i zrzut bazy danych nadal idą pierwsze, pozostałe kontenery do zatrzymania wyłączają się i wracają razem z nim, a polecenie po kopii działa, gdy tylko kontener znów działa. Aktualizacja po kopii nadal czeka, aż kopia się skończy. Jego przebiegi pokazują **Migawka** i ile sekund kontener był wyłączony. Gdyby BombVault zatrzymał się w środku dowolnej kopii kontenera, przy następnym starcie uruchomi z powrotem kontenery, które trzymał wyłączone.

## Aktualizacje po kopii {#updates}

- **Aktualizuj kontenery po udanej kopii zapasowej** pobiera nowszy obraz dopiero po zakończeniu poprzedzającej go kopii, więc świeży punkt przywracania zawsze istnieje wcześniej. Jest domyślnie wyłączone, a pojedynczy kontener można ustawić inaczej na jego stronie w **Aktualizuj po udanej kopii zapasowej**. Na nowy obraz przechodzi tylko to, co sam ustawiłeś: zmienne, etykiety, polecenie, użytkownik i healthcheck, które nadal wyglądają jak w starym obrazie, idą za nowym. Jeśli nowego obrazu nie da się utworzyć ani uruchomić, kontener wraca na obraz, na którym działał, aktualizacja jest pokazana jako nieudana, a następna kopia próbuje ponownie. Kontener, który przez to nie działa, jest wymieniony w przebiegu kopii i w powiadomieniu.
- **Usuń stary obraz po aktualizacji** usuwa obraz, który kontener opuścił. Obraz bazowy, który współdzielą inne kontenery, nigdy nie jest usuwany.
- **Odśwież status aktualizacji Unraid po zaktualizowaniu kontenera** czyści baner aktualizacji Unraid dla tego kontenera.

Wiadomość za każdy zaktualizowany kontener ustawia się w **Ustawienia, Powiadomienia** przez **Powiadamiaj o aktualizacji kontenera**.

## Bezczynność przed kopią {#idle}

Karta **Bezczynność przed kopią** ustawia, kiedy aplikacja uchodzi za bezczynną, dla kontenera, którego zaplanowana kopia na to czeka: **CPU poniżej (% rdzenia)**, **Ruch poniżej (Mbit/s)** i **Przez co najmniej (min)**. Serwer multimediów jest bezczynny, gdy nie streamuje, a dla kontenera w sieci hosta liczy się tylko CPU. Samo czekanie ustawia się na każdym kontenerze, zobacz [Gdy aplikacja jest zajęta](backups.md#app-busy).

## Własny plan dla jednego elementu {#own-plan}

Każdy element ma **Własny plan** w karcie **Harmonogram i miejsce docelowe** na swojej karcie Ustawienia. Z planem w tym miejscu element działa o własnych porach i opuszcza pełną kopię oraz plan swojej sekcji. Bez niego idzie za nimi.

## Przykład: trzy plany, które do siebie pasują {#example}

Typowy serwer Unraid: większość kontenerów da się kopiować w nocy, maszyny wirtualne są duże i zmieniają się powoli, a Immich dostaje nowe zdjęcia przez cały dzień.

1. **Pełna kopia o 03:00.** Otwórz **Ustawienia, Harmonogram**. W **Pełna kopia** ustaw **Kiedy działają kopie** na **Codziennie** o `03:00`. Włącz **Nadrabiaj pominięte kopie zapasowe po starcie**, żeby noc z wyłączonym serwerem została nadrobiona.
2. **Maszyny wirtualne raz w tygodniu.** W **Harmonogramy według sekcji** naciśnij **Wyjątek**, wybierz **Maszyny wirtualne** i ustaw ich plan na **Co tydzień**, **Dni** niedziela, **Godzina** `04:00`. Karta mówi teraz, że wszystkie pozostałe sekcje działają w pełnej kopii.
3. **Maszyny wirtualne kopiują poza siedzibę też w niedzielę.** W **Kopie poza siedzibą według sekcji** naciśnij **Wyjątek**, wybierz **Maszyny wirtualne** i ustaw **Co tydzień**, niedziela o `06:00`, gdy ich kopia zdąży się skończyć.
4. **Immich co 6 godzin.** Otwórz **Kopie zapasowe**, potem `immich`, a potem jego kartę **Ustawienia**. Ustaw **Własny plan** na **Cron** z `0 */6 * * *`.

Co działa teraz:

| Kiedy | Co |
|---|---|
| Codziennie o 03:00 | Kontenery poza `immich`, flash, foldery, zbiory danych ZFS i autokopia, jedna sekcja po drugiej |
| Niedziela o 04:00 | Maszyny wirtualne |
| Niedziela o 06:00 | Kopia maszyn wirtualnych poza siedzibę |
| 00:00, 06:00, 12:00 i 18:00 | `immich` osobno |

Karta **Tydzień** pokazuje linię z 03:00 w każdym dniu, kreski z 04:00 i 06:00 w niedzielę oraz cztery kreski dziennie dla `immich`. `immich` nie jest kopiowany drugi raz o 03:00, bo element z własnym planem opuszcza pełną kopię. Kopie pozostałych sekcji nadal idą zaraz po każdej kopii.

Gdy później usuniesz własny plan `immich`, wróci do przebiegu o 03:00 razem ze swoją sekcją.
