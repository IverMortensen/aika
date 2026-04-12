1. Top down approach, jeg er ikke så interesert i selve databehandling på edge nodene, for min del er det helt vilkårlig hvilken data som blir prosessert.
    Det er veldig enkelt å bli stuck i low-level detaljer hvis man begynner med Agentene. Det er egentlig ikke viktig når det kommer til selve arkitekturen
2. Jeg begynner med RAFT nodene
3. Deretter Local controller




For start av testingen:

1. Vi kan starte med 3 raft noder.
    Disse 3 raft nodene må kommunisere sammen, de må alle ha samme log som lederen, slik at de kjapt kan ta over om lederen feiler. Loggen kan piggybackes sammen med pings.
    Det skal ikke være mange raft noder 3-5 stk. Så Hver raft node kan lagre addressen til de andre nodene, man kan her bruke en dict for å lagre addressen til nodene og en True / False om noden er lederen for raft nodene.

    Leader will make decisions

    Leader can initially be selected based on its ID (Lowest number wins)

    vi trenger terms

    Vi må håndtere elections




RAFT NOTES
Election:
    - Når servere starter opp begynner alle som followers.
    - En node er i follower state så lenge den får gyldige kall fra lederen eller kandidaten.
    - Hvis en node ikke får kommunikasjon over en viss mengde tid så starter noden en ny election
        - Når en node begynner en election så inkrementerer den sin term og sender ut requestVote kall. En node gjør dette til en av tre ting skjer.
        a) Noden vinner election
        b) en annen server velger seg selv som lever
        c) en periode går uten at en node vinner.
    - en node vinner en election hvis den har majoriteten av stemmene
    - En node vil bare stemme på en kandidat per term.
    - Når en node har vunnet sender den ut heartbeats til alle andre noder og sier at jeg er den nye lederen.

    - Mens kandidat noden venter på stemmer, kan kandidat noden motta en AppendEntries RPC fra en annen server som sier at den er lederen. Hvis denne påståtte "lederen" sin term is like stor som kandidat noden sin term vil kandidat noden annerkjenne lederen som gyldig og returnere som følger, hvis RPC kallet er mindre enn kandidaten sitt term vil kandidaten ikke annerkjenne lederen og fortsette i kandidat modus.

    - Hvis det er ingen vinner så vil kandidat nodene få en timeout (random lengde her) og starte en ny election og inkrementere sin term count. Election timeouts are random (150-300ms) 

    Q:
    Er det sånn at en kandidat sender ut en ny term og spør om å bli leder, også svarer bare noden "ok jeg stemmer på deg". Hva hvis den noden som stemte fortsatt har kontakt med leder fra den forrige term?

    VIKTIG!!
        Lederen sender (pusher) heartbeats til alle sine følgere og følgerne responderer med 200 ok, dette må sjekkes litt dypere i, det har noe med append_entries å gjøre. 


    Lederen sender appendEntries (POST) kall til alle sine følgere, Når følgerene får en post request så timer de ikke ut.

    1. Begynn med selve leader election.
    2. Append entries.



Problemet er at kandidat raft nodene rejecter den nye lederen siden den har en lavere term en kandidatene, sjekk opp om dette




1. holde en liste over alle local controllersene (kanskje agentene)
2. ha executable filen / filstien for å restarte filene
3. ha et startup skript for raft + Iver aika
4. legge opp et API for heartbeats
5. Legge til addressen til 



Vi må ha en json dict over alle local controlleren med status 1 eller 0. Hvis statusen er 0 så er den død, er den død så sender vi RPC sånn at den cluster controlleren blir kjørt på nytt.

{
    localController1: 1
    localController2: 1
    localController3: 0
    localController4: 1
}

localController 3 har ikke sendt heartbeat. Da skal RAFT lederen sende RPC: Kjør denne på nytt.


FAKTISK DØD: ssh ttr042@c7-24 "pkill -9 -f inf3203_startupRaftNodes"

ssh USER@c7-24 "pkill -9 -f inf3203_startupRaftNodes"



1. Når en cluster kontroller dør, så skal lederen prøve å recovere den, samme mekanikk kan brukes for å gjennopplive local kontrollerene.
2. Vente på startup skriptet til Iver
3. Raft noden får en liste over noder som kan være local kontrollere
4. Raft skal bruke en python funksjon for å starte opp disse local kontrollerne.
