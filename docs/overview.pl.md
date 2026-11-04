# Przegląd

Przegląd to pierwsza strona paska bocznego i ta, którą warto sprawdzać każdego ranka. Jednym spojrzeniem odpowiada na dwa pytania: czy wszystko jest skopiowane i czy coś wymaga Twojej uwagi. Zaczyna się od nagłówka stanu i czterech kart, a kolejne karty czekają pod **Dostosuj**.

![Przegląd po kilku dniach: nagłówek stanu, Wymaga uwagi z jednym ostrzeżeniem, ochrona każdej sekcji, miejsca docelowe i historia.](assets/screenshots/overview/home-dark.png#only-dark)
![Przegląd po kilku dniach: nagłówek stanu, Wymaga uwagi z jednym ostrzeżeniem, ochrona każdej sekcji, miejsca docelowe i historia.](assets/screenshots/overview/home-light.png#only-light)

## Nagłówek stanu {#status-head}

Nagłówek leży na górze strony i zawsze tam zostaje. Jego pierścień pokazuje, ile jest chronione, na przykład **41 z 43 chronionych**, a linia obok mówi, jak wyglądają sprawy:

| Nagłówek mówi | Co to znaczy |
|---|---|
| **Wszystko skopiowane** | Każdy element każdej włączonej sekcji ma aktualną kopię. |
| **Skopiowano, z uwagami** | Wszystko jest skopiowane, a niektóre przebiegi zostawiły uwagę, na przykład o plikach, których nie dało się odczytać. |
| **Nie wszystko jest jeszcze chronione** | Niektóre elementy nie mają aktualnej kopii, na przykład czekają na swoją pierwszą. |
| **3 rzeczy wymagają uwagi** | Coś się nie udało albo czeka na decyzję. Wymienia to karta **Wymaga uwagi**. |
| **Trwa tworzenie kopii** | Kopia trwa. Pierścień zmienia się w pierścień postępu ze stacjami **Zatrzymanie**, **Odczyt** i **Uruchomienie**, a pełna kopia liczy swoje elementy na bieżąco, na przykład **Trwa tworzenie kopii: 12 z 43**. **Pokaż przebieg** otwiera ten przebieg. |
| **Trwa kopiowanie poza siedzibę** | Trwa kopiowanie do miejsca docelowego. Nie pokazuje procentu, bo `restic copy` go nie podaje. |

Pod linią widać, kiedy był ostatni przebieg i kiedy startuje następny zaplanowany.

## Wymaga uwagi {#needs-you}

Ta karta wymienia to, na co warto spojrzeć, od najnowszego, a gdy lista jest pusta, mówi **Nic do zrobienia.** Każdy wiersz ma własne wyjście:

- **Nieudana kopia** podaje element i powód. **Pokaż błędy** pokazuje własne wyjście restic, a **Spróbuj ponownie** uruchamia kopię jeszcze raz.
- **Kopiowanie poza siedzibę nie powiodło się** podaje sekcję i od kiedy. **Replikuj teraz** próbuje ponownie.
- **Anomalia w plex** prowadzi do znaleziska przez **Sprawdź**. Dopóki otwarte jest znalezisko krytyczne, przechowywanie zachowuje stare kopie tego elementu.
- **Nowe** elementy, które BombVault znalazł i których nic jeszcze nie kopiuje, na przykład świeżo zainstalowany kontener, oferują **Zacznij tworzyć kopie** albo **Nie kopiuj**.
- **Kopia, która się poddała**, bo jej aplikacja była zajęta przez cały czas oczekiwania, zobacz [Gdy aplikacja jest zajęta](backups.md#app-busy).
- **Nieudana replika ZFS**, wyjaśniona zwykłymi słowami, na przykład że serwer nie ma polecenia zfs, z wyjściem serwera pod **Pokaż**.
- **Kopie zapasowe stoją: klucza nie da się już otworzyć** pojawia się, gdy ten serwer przejął klucz innego serwera, a `APP_KEY` zmienił się potem. Zobacz [Rozwiązywanie problemów](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Zapisz swój zestaw odzyskiwania** zostaje, dopóki nie pobierzesz zestawu i nie naciśniesz **Bezpiecznie zapisałem**.

## Stan ochrony {#protection-status}

Każda włączona sekcja ma wiersz z czterema kontrolami:

| Kontrola | Robi się zielona, gdy | Inne słowa, które możesz zobaczyć |
|---|---|---|
| **Lokalne** | ostatnia kopia jest aktualna | brak kopii, brak harmonogramu, ostatni przebieg się nie udał, kopia spóźniona, nie każdy element jest chroniony, kopia wkrótce |
| **Poza siedzibą** | ostatnia kopia poza siedzibą jest aktualna | brak kopii poza siedzibą, kopiowanie trwa, kopia spóźniona, kopiowanie wstrzymane, jeszcze bez kopii, replika się nie udała |
| **Ochrona przed usunięciem** | test manipulacji dowiódł, że druga strona odmawia usuwania | brak, brak i ryzyko zaakceptowane, jeszcze nie udowodniona, test się nie udał, nie udowodniona, bo jej kontrola jest wyłączona |
| **Testowe odtwarzanie** | przywracanie z kopii poza siedzibą się powiodło | nigdy nie uruchomione, trwa, nie powiodło się, spóźnione, tylko ręcznie, niemożliwe dla tej sekcji |

Sekcja jest zielona tylko na podstawie sprawdzonych faktów. Kopia na tym serwerze nie liczy się jako poza siedzibą, a miejsce oznaczone **Akceptuj bez ochrony przed usunięciem** pokazuje szary pierścień zamiast bursztynowej kropki, więc kolor sekcji wynika z jej pozostałych kontroli.

Sekcja z harmonogramem pokazuje **Jeszcze bez kopii zapasowej** i pozostaje neutralna, dopóki nie nadejdzie termin jej pierwszego przebiegu. Potem pokazuje **Zaległe** i alarmuje jeden raz. Sekcja, która nie ma czego kopiować, bo żaden harmonogram nie obejmuje żadnego z jej elementów, pokazuje **Bez harmonogramu** i nigdy nie jest zaległa.

Kliknięcie sekcji otwiera jej okno z opisem każdej kontroli, policzonymi elementami i przyciskiem **Uruchom testowe odtwarzanie** albo **Uruchom testowe odtwarzanie ponownie**. Pokazuje też **Reguła przycinania ustawiona** albo **Brak reguły przycinania**, a reguła przechowywania własnego folderu sekcji liczy się tak samo jak jej reguły poza siedzibą.

![Okno ochrony sekcji Containers z opisanymi czterema kontrolami.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Okno ochrony sekcji Containers z opisanymi czterema kontrolami.](assets/screenshots/overview/protection-window-light.png#only-light)

## Miejsca docelowe {#storage-locations}

Każde miejsce docelowe stoi tu jako naczynie ze swoim poziomem zapełnienia, tempem wzrostu, na przykład **Rośnie o 4,2 GB/tydz.**, oraz informacją, kiedy byłoby pełne przy tym tempie, na przykład **pełne za około 31 tygodni**. S3, B2 i rest-servery nie podają, ile miejsca zostało, więc pokazują **Wolne miejsce nieznane** i tylko swój wzrost. Kliknięcie prowadzi do strony miejsca. Zobacz [Miejsca docelowe](offsite-recovery.md#destinations).

## Historia {#history}

**Historia** ma dwie karty. **Przebiegi** wymieniają każdy przebieg z wynikiem, czasem trwania i powodem, a kliknięcie go otwiera. **Dziennik** pokazuje każdą linię z jej czasem, tak jak zapisuje ją BombVault, i można go zawęzić do jednego dnia. Linia robi się pomarańczowa, gdy kopia przeszła, ale coś pominęła, i mówi, co pominęła.

## Więcej kart pod Dostosuj {#customize}

**Dostosuj** na górze strony pozwala ułożyć karty. Przeciągnij kartę za pasek, żeby ją przesunąć, zmień jej rozmiar kropką w dolnym rogu albo ją ukryj. Ukryte karty czekają pod **Ukryte karty**, a **Pokaż** przywraca jedną z nich. Podczas układania karty lekko się kołyszą, a **Gotowe** to kończy. Układ jest zapisywany w tej przeglądarce.

Te karty są domyślnie ukryte:

- **Anomalie**: otwarte znaleziska, zobacz [Anomalie](anomalies.md).
- **Bez kopii zapasowej**: wszystko na tym serwerze, czego nie obejmuje żadna zaplanowana kopia, każde z powodem, na przykład **nigdy nie dodane**, **nieuwzględnione w harmonogramie** albo **wszystkie miejsca docelowe jego sekcji są wyłączone**. Kontenery czyta prosto z Dockera, więc znajduje też taki, którego BombVault jeszcze nie widział.
- **Kalendarz**: ostatnie 90 dni, jeden kwadrat na dzień. Kliknięcie dnia pokazuje jego linie w dzienniku pod **Historia**.
- **Magazyn**: ile miejsca zajmuje każda sekcja i jak dobrze deduplikują się jej dane.
- **Przegląd**: najstarsza migawka we wszystkich sekcjach, kiedy ruszy następna kopia i jak poszła ostatnia.
- **Statystyki**: ile jest kontenerów i maszyn wirtualnych, ile błędów i ile planów jest aktywnych lub wstrzymanych.
- **Integracja z hostem**: co BombVault osiąga i czego potrzebuje na hoście, na przykład gniazdo Docker, libvirt, restic i rclone. Zacznij tutaj, gdy coś wydaje się niepodłączone.

![Tryb Dostosuj: karty można przeciągać, zmieniać im rozmiar i ukrywać, a ukryte czekają niżej.](assets/screenshots/overview/customize-dark.png#only-dark)
![Tryb Dostosuj: karty można przeciągać, zmieniać im rozmiar i ukrywać, a ukryte czekają niżej.](assets/screenshots/overview/customize-light.png#only-light)

## Jak liczy BombVault {#counting}

Każda karta i nagłówek liczą tak samo: elementy włączonych sekcji, które są nadal zainstalowane i których nie oznaczyłeś jako **Nie kopiuj**. Element liczy się jako chroniony tylko wtedy, gdy sięga go harmonogram i ma kopię. Sama stara kopia się nie liczy, więc kontener, który wyjąłeś z harmonogramu, pokazuje się jako niechroniony, dopóki go nie przywrócisz albo nie oznaczysz jako **Nie kopiuj**.

## Pierwsze kroki {#first-steps}

Na nowej instalacji Przegląd zaczyna się od karty **Pierwsze kroki**: **Wybierz sekcje**, **Dodaj miejsce docelowe** i **Pierwsza kopia zapasowa**. Zostaje, dopóki pierwsza kopia się nie zakończy. [Pierwsze kroki](getting-started.md#first-run) przeprowadzają przez nią krok po kroku.
