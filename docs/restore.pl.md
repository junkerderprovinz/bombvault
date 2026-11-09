# Przywracanie

Każde przywracanie zaczyna się na stronie **Przywracanie**: wybierz sekcję, wyszukaj to, co chcesz odzyskać, i wskaż kopię. Jedno okno przywraca każdy rodzaj elementu i pyta krok po kroku. Domyślnie nic na oryginale się nie zmienia, bo kopia wraca obok niego.

![Strona Przywracanie z sekcjami jako kafelkami, wyszukiwarką i kartą Stacki.](assets/screenshots/restore/page-dark.png#only-dark)
![Strona Przywracanie z sekcjami jako kafelkami, wyszukiwarką i kartą Stacki.](assets/screenshots/restore/page-light.png#only-light)

## Strona Przywracanie {#page}

- **Sekcje** wymieniają sekcje jako kafelki, każdy z liczbą elementów, które mają migawki.
- **Co chcesz przywrócić?** przeszukuje elementy wybranej sekcji. Każdy wiersz pokazuje swoją najnowszą migawkę, a **Otwórz wpis** prowadzi do strony elementu.
- **Znajdź plik** szuka pliku lub folderu po nazwie w każdej kopii zapasowej, zobacz [Szukanie pliku](#find-a-file).
- **Stacki** wymieniają projekty Compose z co najmniej dwoma skopiowanymi kontenerami, na przykład `immich` z jego bazą danych i Redis. Wyszukiwarka znajduje stack po nazwie albo po któremkolwiek z jego kontenerów.
- **W toku** pokazuje przywracanie, gdy trwa, także po zamknięciu jego okna.
- **Awaria** uruchamia Kreator awaryjny dla serwera, którego już nie ma albo który postawiono od nowa, zobacz [Kreator awaryjny](offsite-recovery.md#emergency).
- **Inne źródła** zawierają **Ze starego folderu z kopiami**, dla kopii innego BombVault, **Import z wtyczki Appdata.Backup** oraz **Zadania z Duplicati lub Duplicacy**.

![Strona Przywracanie w telefonie.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Strona Przywracanie w telefonie.](assets/screenshots/restore/page-phone-light.png#only-light)

## Okno przywracania {#window}

Kliknięcie elementu otwiera **Przywróć** z jego nazwą. Okno pyta krok po kroku, a każdy krok można zmienić, zanim zaczniesz:

1. **Kopia zapasowa**: zalecana migawka jest wybrana z góry. To najnowsza albo, po otwartej anomalii, ostatnia sprzed niej. Kopia zrobiona przed wcześniejszym przywróceniem zawiera stan, który to przywrócenie zastąpiło, więc jest pomijana, chyba że nie ma innej. Starsze są na liście, a **Porównaj** pokazuje, czym różnią się dwie z nich.
2. **Źródło**: skąd pochodzą pliki, gdy migawka leży w kilku miejscach. **Lokalne** jest najszybsze.
3. **Co**: **Wszystko** albo **Pojedyncze pliki**, gdzie zaznaczasz pliki i foldery, które chcesz.
4. **Dokąd**: cel. Wybór zależy od rodzaju elementu:

    | Cel | Co robi |
    |---|---|
    | **Obok oryginału** | Domyślny. Kontener wraca jako drugi kontener o nazwie `<name>-restored` z własnymi portami, dyski i NVRAM maszyny wirtualnej trafiają do folderu `<folder>-restored`, a zestaw folderów wraca jako `<path>-restored`. Oryginał się nie zmienia. |
    | **Zastąp oryginał** | Nadpisuje to, co jest teraz. Z **Najpierw zrób kopię obecnego stanu**, włączonym domyślnie, BombVault najpierw kopiuje dzisiejszy stan, żebyś mógł wrócić. Jeśli ta kopia się nie uda, przywracanie nie startuje. Działa to też dla kontenera lub maszyny wirtualnej, których nie ma już na serwerze: foldery, które przywracanie nadpisuje, dostają własną kopię. |
    | **Do folderu** | Tylko pliki, do przejrzenia. |
    | **Pobierz jako ZIP** | Dysk flash, na nowy pendrive. BombVault nigdy nie zapisuje na działający dysk flash. |
    | **Do nowego zbioru danych** i **Przywróć do samego zbioru danych** | Zbiory danych ZFS, zobacz [Zbiory danych ZFS](zfs-datasets.md#restore). |

5. **Co się stanie** mówi słowami, co zrobi przywracanie, a scena pokazuje wybrany cel.

Zanim **Przywróć** się odblokuje, okno sprawdza, czy repozytorium odpowiada, czy klucz je otwiera, czy migawka istnieje i czy cel ma miejsce. Dopóki któraś kontrola zawodzi, przycisk pozostaje zablokowany, a (i) w nim mówi która.

Okno pokazuje też, co przywracanie zmienia w porównaniu z tym, co jest teraz: nowe, zastąpione i niezmienione pliki, z listą na życzenie, oraz pliki w celu, których nie ma w kopii i które zostają, gdzie są. restic wylicza to jako próbny przebieg z rozmiaru i czasu modyfikacji, bez czytania plików, a bardzo duże drzewo zatrzymuje się po 30 sekundach i mówi o tym. Dla kontenerów i maszyn wirtualnych porównuje ustawienia, które przywracanie tworzy na nowo, z działającymi: obraz i tag, porty, nazwy zmiennych i wolumeny albo pamięć, vCPU, dyski i sieć.

Niektóre rzeczy mogą zatrzymać przywracanie, zanim cokolwiek zmieni. Statyczny IP kontenera i opublikowane porty muszą być wolne, a jego sieć musi istnieć. Na Unraid każde przywracanie na miejscu odmawia folderu, który nie leży na zamontowanej puli lub udziale, bo pliki trafiłyby do pamięci i zniknęły po restarcie. Przywracanie na miejscu wymienia każdy inny kontener, którego bind mount sięga do folderu, do którego zapisuje, na przykład „ta ścieżka jest używana także przez nextcloud-db”, ale to tylko ostrzega.

Kilka opcji pojawia się tam, gdzie mają zastosowanie: **Zostaw zatrzymane po przywróceniu**, żeby obejrzeć kontener lub maszynę wirtualną, zanim znów ruszą, oraz **Przywróć bez GPU i środowiska uruchomieniowego** na hoście, któremu brakuje GPU lub środowiska, z którym zrobiono kopię.

Przywracanie działa w tle. Możesz zamknąć okno, a postęp widać też pod **W toku**. Długie przywracanie pokazuje procent i można je anulować, a anulowane przywracanie jest zapisane jako anulowane, nie jako nieudane. Przywracanie anulowane w trakcie zatrzymywania kontenera albo takie, które nie może usunąć kontenera, jeszcze niczego nie przywróciło, więc kontener startuje ponownie. Kontener, który bierze wolumeny od innego przez `--volumes-from`, zachowuje je, a przywracanie nie zacznie się, dopóki tamten kontener nie istnieje.

![Okno przywracania w kroku Co, z zaznaczonymi pojedynczymi plikami.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Okno przywracania w kroku Co, z zaznaczonymi pojedynczymi plikami.](assets/screenshots/restore/window-what-light.png#only-light)

### Dokąd trafia przywracanie do folderu {#folders}

**Foldery przywracania** w **Ustawienia, Przywracanie** ustawiają **Domyślny folder przywracania**, w razie potrzeby z **Podfolder dla każdej sekcji**, oraz **Wyjątek** dla jednej sekcji. Wybrane pliki trafiają prosto do folderu docelowego: jeden plik wraca jako sam ten plik, folder jako folder z zawartością, a wybory z różnych folderów zachowują tylko swoją ścieżkę pod wspólnym folderem. BombVault odmawia celu, który nie leży na zamontowanej pamięci, bo na Unraid taki folder żyje w pamięci i znika po restarcie, i nigdy nie przywraca do własnego folderu danych.

## Przykład: odzyskaj jeden plik {#example-single-file}

Ktoś wczoraj edytował `config.php` Nextcloud i Nextcloud już nie startuje. Nocna kopia z 03:00 nadal ma dobry plik. Chcesz ten jeden plik, a nie cały kontener.

1. Otwórz **Przywracanie**, wybierz kafelek **Containers** i wpisz `nextcloud` w **Co chcesz przywrócić?**.
2. Kliknij `nextcloud`. Okno otwiera się z kopią z 03:00 wybraną jako zalecana.
3. Zostaw **Źródło** na **Lokalne**.
4. W **Co** wybierz **Pojedyncze pliki**. Otwórz foldery aż do `config` i zaznacz `config.php`.
5. W **Dokąd** wybierz **Do folderu** i wskaż folder, na przykład `user/restore`, czyli `/mnt/user/restore` na hoście.
6. Przeczytaj **Co się stanie**: plik ląduje w folderze docelowym, a kontener się nie zmienia. Naciśnij **Przywróć**.

Okno kończy się komunikatem **Gotowe: pliki są w /mnt/user/restore**. Plik znajdziesz jako `/mnt/user/restore/config.php`, bez folderów nad nim. Porównaj go z uszkodzonym, skopiuj z powrotem do folderu appdata i uruchom Nextcloud ponownie.

Żeby zamiast tego od razu odłożyć plik na miejsce, wybierz w kroku 5 **Zastąp oryginał**. Zostaw włączone **Najpierw zrób kopię obecnego stanu**, żeby BombVault skopiował uszkodzony stan, zanim cokolwiek zapisze, wpisz `nextcloud` dla potwierdzenia i naciśnij **Przywróć**. Nextcloud jest zatrzymany, gdy wybrany plik wraca na swoje miejsce, a potem startuje ponownie, i zapisywane są tylko zaznaczone pliki. Przywracanie, które nie powiedzie się w połowie, zostawia Nextcloud zatrzymany, bo plik, który czyta, może być zapisany tylko w połowie, a przebieg to mówi.

![Okno przywracania w kroku Co się stanie, z wybraną opcją Zastąp oryginał.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Okno przywracania w kroku Co się stanie, z wybraną opcją Zastąp oryginał.](assets/screenshots/restore/window-replace-light.png#only-light)

## Szukanie pliku {#find-a-file}

Gdy znasz nazwę pliku, ale nie wiesz, który wpis albo która kopia wciąż go ma, wpisz nazwę w **Znajdź plik** i naciśnij **Szukaj**. Zwykły tekst pasuje w dowolnym miejscu nazwy i nie zwraca uwagi na wielkość liter, więc `invoice` znajdzie `Invoice-2024.pdf`. Z `*` albo `?` wzorzec musi pasować do całej nazwy, więc `*.pdf` znajdzie każdy plik PDF. Wpisz samą nazwę, bez folderu.

Wyszukiwanie czyta każdą kopię kontenerów, zestawów folderów, zbiorów danych ZFS i dysku flash. Każde trafienie pokazuje wpis, ścieżkę i każdą kopię, która zawiera plik, od najnowszej, z datą, rozmiarem i miejscem docelowym, z którego ją odczytano. Kopia, w której plik ma ten sam rozmiar i czas modyfikacji co w poprzedniej, ma oznaczenie **bez zmian**, więc widać, gdzie plik zmienił się ostatnio. Kliknięcie kopii otwiera na niej okno przywracania, z wybraną opcją **Pojedyncze pliki** i zaznaczonym plikiem. Plik z dysku flash wraca jako część pliku ZIP dysku flash, bo dysk flash przywraca się tylko w całości.

Wyszukiwanie chwilę trwa, bo restic przechodzi przez każdą kopię. **Zatrzymaj** je kończy, podobnie jak opuszczenie strony. Pod polem widać, które miejsca docelowe zostały odczytane:

- Miejsca docelowe na tym serwerze i w twojej sieci są przeszukiwane zawsze.
- Kopia, która leży w kilku miejscach docelowych, jest czytana tylko raz, w pierwszym z nich. Miejsce docelowe, którego wszystkie kopie odczytano gdzie indziej, ma oznaczenie **już przeszukane w innym miejscu docelowym**.
- Miejsca docelowe poza siedzibą, takie jak S3, B2 czy SFTP, są pomijane, chyba że włączysz **Przeszukaj też miejsca docelowe poza siedzibą**. Wtedy czytane są tam tylko kopie, których nie ma żadne miejsce docelowe tutaj, zwykle starsze, które dłuższe przechowywanie trzyma poza siedzibą. Pierwsze wyszukiwanie w takim miejscu może długo trwać, a niektórzy dostawcy naliczają opłaty za pobrane dane.
- Miejsce docelowe, którego nie udało się otworzyć, ma oznaczenie **nie udało się odczytać**, a (i) obok mówi dlaczego. Pozostałe miejsca docelowe są przeszukiwane dalej.

Maszyny wirtualne są zapisywane jako całe dyski, więc pliki w nich nie są przeszukiwane. Autokopia i kopie innego BombVault w **Inne źródła** również nie są przeszukiwane. Wyszukiwanie wymienia pierwszych 200 plików, więc przy krótkiej nazwie, takiej jak `log`, może trzeba będzie dopisać kilka liter.

## Co wraca, według rodzaju {#kinds}

**Kontenery** są tworzone na nowo z zapisanej definicji przez API Docker, z entrypointem, nazwą hosta, ustawieniami zatrzymania, wolumenami i montowaniami, limitami zasobów, sterownikiem logów, ustawieniami DNS i starymi linkami. Kontener wraca na obrazie, na którym działał w chwili kopii, a nie na nowszym pobranym później, a brakujący obraz jest najpierw pobierany, z logowaniem z **Rejestry kontenerów** w **Ustawienia, Przywracanie**. Wraca też jego GPU lub środowisko (`--gpus`, `--runtime=nvidia`). Na hoście bez tego sterownika lub środowiska przywracanie mówi o tym i oferuje **Przywróć bez GPU i środowiska uruchomieniowego**. Link do kontenera, którego brakuje albo który jest zatrzymany, gdy przywrócony startuje, jest pomijany, a historia przebiegu to mówi. Przywracanie, które zawodzi po usunięciu kontenera, odkłada go z powrotem, zatrzymanego.

Kopia może zawierać foldery, których kontener nie ma już zaznaczonych, na przykład starsza kopia sprzed zniknięcia folderu. Folder, którego nie ma albo który jest pusty, wraca. Folder, w którym są teraz pliki, zostaje taki, jaki jest, bo mogłeś go odznaczyć celowo, a kontrola przed przywróceniem i historia przebiegu go wymieniają. Odzyskasz go przez **Pojedyncze pliki** albo **Do folderu**.

**Maszyny wirtualne** dostają ponownie zdefiniowany XML przez SSH oraz podłączone dyski i UEFI NVRAM, nawet po usunięciu maszyny. Zobacz [Kopie maszyn wirtualnych](vm-backup-ssh-setup.md) dla maszyn z migawkami Unraid i dyskami na zvol.

**Zestawy folderów** wracają z właścicielami, uprawnieniami i znacznikami czasu, tak jak zapisuje je restic. Gdy cel odmawia tylko właściciela, jak robi to `/mnt/user`, przebieg kończy się uwagą, która o tym mówi. BombVault porównuje wtedy pliki z kopią, a plik, który nie dotarł, sprawia, że przywracanie się nie udaje.

**Dysk flash** nigdy nie dotyka działającego `/boot`. Przywracanie to pobranie pliku `flash-<id>.zip`, gotowego do wrzucenia w Unraid USB Creator. ZIP zawiera `APP_KEY`, więc jego pobranie wymaga hasła logowania i zalogowania na samej instancji. Okno wymienia też wtyczki w każdej kopii flash z ich wersją i rozmiarem i potrafi odłożyć jedną z nich na działający dysk flash: jej plik `.plg`, jej folder w `config/plugins` i pliki pakietów, które ma kopia. Unraid instaluje ją przy następnym uruchomieniu albo od razu w **Plugins, Install Plugin**.

**Zbiory danych ZFS** mają własne cele i zachowują swoje właściwości ZFS, zobacz [Przywracanie](zfs-datasets.md#restore).

**Autokopia** wraca tylko przez Kreator awaryjny, który restartuje BombVault, więc działająca baza nigdy nie jest nadpisywana pod otwartym uchwytem.

**Zrzuty baz danych** nigdy nie są wczytywane z powrotem same. Wiersz pod **Zrzuty bazy danych** można pobrać, zapisać do folderu albo zaimportować do świeżo uruchomionej bazy. Zobacz [Zrzuty baz danych](backups.md#database-dumps).

## Przywracanie kilku elementów naraz {#several}

Na stronie Przywracanie wybieraj elementy tak jak wiersze na liście Kopie zapasowe: Ctrl+kliknięcie, Shift+kliknięcie, Ctrl+A, długie przytrzymanie albo **Zaznacz**. **Przywróć** otwiera wtedy jedno okno dla wszystkich.

- **Która kopia zapasowa**: **Zalecana** bierze dla każdego elementu kopię, którą zaleciłoby jego własne okno. **Jeden moment dla wszystkich** bierze dla każdego elementu jego najnowszą kopię z wybranego **Moment** albo wcześniejszą, na przykład z wieczora przed tym, jak coś poszło źle.
- **Dokąd**: **Obok oryginału** albo **Zastąp oryginał**, z **Najpierw zrób kopię obecnego stanu**.

Każdy element jest sprawdzany tak jak pojedyncze przywracanie, a potem idą jeden po drugim. Każdy wiersz pokazuje, jak poszło jemu: **Czeka**, **W toku**, **Przywrócono**, **Niepowodzenie** albo **Pominięto**. **Zatrzymaj po tej** pozwala skończyć trwające przywracanie i nie startuje kolejnych. Dysk flash przychodzi jako ZIP do pobrania, a zbiory danych ZFS i autokopia są przywracane po jednym.

![Przywracanie kilku elementów: jeden wiersz na element z jego postępem.](assets/screenshots/restore/several-dark.png#only-dark)
![Przywracanie kilku elementów: jeden wiersz na element z jego postępem.](assets/screenshots/restore/several-light.png#only-light)

## Przywracanie stacku {#stacks}

Kliknięcie stacku na karcie **Stacki** otwiera **Przywróć stack**. Przywraca każdego członka z jego ostatniej kopii jednocześnie, a z **Uruchom w kolejności zależności po przywróceniu** startuje ich w kolejności `depends_on`: najpierw bazy danych i pamięci podręczne, potem aplikację. Gdy folderu projektu członka brakuje w wybranym miejscu, okno oferuje zamiast niego stan z Lokalne.

BombVault najpierw sprawdza członków i folder projektu, a przywracanie, które nie mogłoby się udać, niczego nie zmienia. W przeciwnym razie zatrzymuje działających członków, aplikacje przed ich bazami danych, potem zapisuje folder projektu, a potem przywraca członków. Jeśli folder projektu nie wróci, członkowie i tak są przywracani, ale zostają zatrzymani, bo folder może być zapisany tylko w połowie, a to w nim zwykle leżą bind mounty członka. Przebieg każdego członka to mówi.

Baza danych, która ma tylko zrzuty bazy, bez kopii plików, i której kontenera i nazwanego wolumenu już nie ma, wraca bez swoich danych. Kontrola mówi o tym przed przywróceniem. Baza zostaje zatrzymana, podobnie jak kontenery, które od niej zależą, żeby żadna aplikacja nie uruchomiła swojej konfiguracji na pustej bazie. Uruchom ją, zaimportuj zrzut i uruchom pozostałe. Zobacz [Zrzuty baz danych](backups.md#database-dumps).

## Inne źródła {#other-sources}

- **Ze starego folderu z kopiami** otwiera repozytorium innego BombVault, który już nie działa, na dysku albo w miejscu docelowym poza siedzibą, z jego zestawem odzyskiwania albo wpisanym `APP_KEY`. Sesja jest tylko do odczytu i żyje wyłącznie w pamięci. Zobacz [Ze starego folderu z kopiami](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Import z wtyczki Appdata.Backup** czyta folder kopii wtyczki. Każde archiwum kontenera staje się punktem przywracania jego kontenera, z datą utworzenia przez wtyczkę. Archiwa zaimportowane wcześniej są pomijane, a same archiwa są tylko czytane. Kontener potrzebuje najpierw jednej kopii w BombVault, żeby przywracanie miało jego definicję. Przechowywanie omija zaimportowane punkty przywracania, więc usuń taki sam, gdy już go nie potrzebujesz.
- Sekcja **Zadania z Duplicati lub Duplicacy** zamienia zadania kopii zapasowych tych narzędzi w zestawy folderów. Odczytuje przy tym tylko ich ustawienia. Stare kopie zostają przy starym narzędziu, a BombVault zaczyna od własnej pierwszej kopii. Żadne z tych narzędzi nie musi być tu zainstalowane.
    - **Z Duplicati**: w Duplicati wyeksportuj każde zadanie do pliku bez hasła i wybierz pliki `.json`. Eksport z hasłami przenosi też dane logowania do miejsca docelowego.
    - **Z Duplicacy**: wybierz folder Duplicacy Web, w którym jest `duplicacy.json`, albo folder, którego kopię robisz z wiersza poleceń Duplicacy i w którym jest `.duplicacy`. Klucze zapisane przez wiersz poleceń w `.duplicacy/preferences` zostaną przeniesione. Duplicacy Web przechowuje swoje dane logowania w postaci zaszyfrowanej, więc okno o nie poprosi.

    Nic nie powstaje, dopóki nie zobaczysz podglądu. Każdy folder źródłowy staje się zestawem folderów z folderem na tym serwerze, w którym go znaleziono, z wykluczeniami, które dało się przełożyć, i z harmonogramem. Jeśli stare narzędzie działało we własnym kontenerze i jakiegoś folderu nie znaleziono, wybierasz go sam. Kopie trafiają do nowego miejsca docelowego w folderze `bombvault` wewnątrz starego miejsca docelowego, gdy jest nim folder lokalny, S3, SFTP, zdalny zasób rclone albo Backblaze B2 przez jego adres S3. Dane logowania z pliku są zapisywane razem z twoimi pozostałymi danymi logowania, a o te, których w pliku brakuje, okno zapyta. Notatki pod każdym zadaniem mówią, co nie zostało przeniesione, na przykład reguły dołączania, wyrażenia regularne, jak długo przechowywano kopie oraz stare hasło szyfrowania, którego BombVault nie potrzebuje.

## Po przywróceniu {#after}

Kopia przywrócona obok oryginału jest wymieniona pod **Kopie zapasowe** jako wykluczona, więc nie liczy się jako otwarte zadanie. Dodaj ją do harmonogramu, jeśli chcesz ją zachować. Kontener lub maszyna wirtualna, które działały w chwili kopii, wracają działające, a zatrzymane zostają zatrzymane. Na Unraid przywrócony kontener pojawia się ponownie w zakładce Docker jako zwykła, edytowalna aplikacja, a maszyna wirtualna w VM Manager z podłączonymi dyskami i NVRAM.
