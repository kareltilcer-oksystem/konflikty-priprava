/**
 * Every user-visible string that the frontend owns, in one place (PRD NFR7), so
 * a copy change never means hunting through components.
 *
 * Not here: the server's error messages. The API emits display-ready Czech and
 * the UI prints `error.message` as it arrives — duplicating those strings would
 * only create two versions to keep in step.
 *
 * Wording is taken from design/v1 and docs/DESIGN-HANDOFF.md §7.
 */
export const cs = {
  app: {
    name: 'Velké konflikty — příprava',
    nav: { problems: 'Problémy', meetings: 'Porady' },
    signIn: 'Přihlásit se',
    signOut: 'Odhlásit',
  },

  login: {
    title: 'Přihlášení',
    username: 'Uživatelské jméno',
    password: 'Heslo',
    submit: 'Přihlásit se',
    cancel: 'Zrušit',
  },

  bucket: {
    title: 'Problémy',
    add: 'Přidat problém',
    search: 'Hledat v problémech…',
    sortLabel: 'Řadit',
    filters: { open: 'nevyřešené', done: 'vyřešené', all: 'vše' },
    sorts: {
      created_desc: 'nejnovější',
      created_asc: 'nejstarší',
      meetings_desc: 'nejčastěji na poradě',
    },
    columns: {
      problem: 'Problém',
      author: 'Autor',
      created: 'Vloženo',
      attachments: 'Přílohy',
      meetings: 'Porady',
    },
    emptyList: 'Žádné nevyřešené problémy.',
    emptyListHint: 'Na příští poradu není co připravovat.',
    emptySearch: 'Hledání nic nenašlo.',
    emptySearchHint: 'Hledá se v názvu i popisu, bez ohledu na diakritiku a velikost písmen.',
    clearSearch: 'Zrušit hledání',
    markDone: 'Označit jako vyřešené',
    markOpen: 'Vrátit mezi nevyřešené',
  },

  problem: {
    detailTitle: 'Detail problému',
    back: 'Problémy',
    // "Vložil(a)" rather than a gendered form: the app knows a display name
    // and nothing else, and inferring gender from a name would be wrong as
    // often as it was right. This is the ordinary Czech convention for it.
    createdBy: 'Vložil(a)',
    updatedAt: 'upraveno',
    edit: 'Upravit',
    delete: 'Smazat',
    markDone: 'Označit jako vyřešené',
    markOpen: 'Vrátit mezi nevyřešené',
    doneBadge: 'Vyřešeno',
    attachments: 'Přílohy',
    addAttachments: 'Přidat přílohy',
    removeAttachment: 'Odebrat přílohu',
    download: 'Stáhnout',
    onMeetings: 'Na poradách',
    timelineEmpty: 'Tento problém zatím nebyl na žádné poradě.',
    itemNumber: 'bod',
    noAction: 'Akce / výsledek — bez zápisu',
  },

  form: {
    createTitle: 'Přidat problém',
    editTitle: 'Upravit problém',
    title: 'Název',
    description: 'Popis',
    descriptionHint: 'Zlomy řádků se zachovají.',
    link: 'Odkaz',
    author: 'Autor',
    authorHint: 'Problém se uloží pod jménem vybraného autora.',
    authorSelf: (name: string) => `${name} (já)`,
    attachments: 'Přílohy',
    dropzone: 'Vložte snímek obrazovky (Ctrl+V) nebo přetáhněte soubory',
    dropzoneHint: 'Jakýkoli typ souboru · max 100 MB na soubor · 20 souborů · 512 MB celkem',
    dropActive: 'Pusťte soubory sem',
    pick: 'Vybrat soubory',
    atomicHint: 'Problém i přílohy se uloží jednou akcí.',
    save: 'Uložit',
    saving: 'Ukládám…',
    cancel: 'Zrušit',
    remove: 'Odebrat',
    uploaded: 'nahráno',
    optional: '— nepovinné',
  },

  meetings: {
    title: 'Porady',
    create: 'Nová porada',
    columns: { date: 'Datum', week: 'Týden', agenda: 'Program' },
    upcoming: 'Připravuje se',
    archived: 'Archiv',
    showArchive: 'Zobrazit archiv',
    archiveHint: 'Porady starší než 365 dní. Nic se nemaže, jejich odkazy fungují dál.',
    empty: 'Zatím nebyla založena žádná porada.',
    nextMeeting: (date: string) => `Nejbližší porada je ${date}.`,
    createdBy: 'vytvořil',
  },

  newMeeting: {
    title: 'Nová porada',
    date: 'Datum porady',
    slugHint: 'Adresa porady se vytvoří automaticky:',
    note: 'Poznámka k celé poradě',
    notePlaceholder: 'Např. kdo se opozdí nebo čím začneme',
    submit: 'Vytvořit poradu',
    cancel: 'Zrušit',
  },

  agenda: {
    back: 'Porady',
    meetingNote: 'Poznámka k celé poradě',
    meetingNotePlaceholder: 'Nepovinná poznámka k celé poradě',
    prepNote: 'Poznámka k přípravě',
    prepNotePlaceholder: 'Co k tomu chceme na poradě říct',
    actionNote: 'Akce / výsledek',
    actionNotePlaceholder: 'Doplní se po poradě',
    editDate: 'Upravit datum',
    saveDate: 'Uložit',
    addProblems: 'Přidat problémy',
    deleteMeeting: 'Smazat poradu',
    removeItem: 'Odebrat z programu',
    dragHandle: 'Přetažením změníte pořadí',
    dragHint: 'Přetahování — poznámky jsou sbalené, aby byl vidět celý program. Pusťte bod na požadované místo.',
    dropAt: (n: number) => `Pustit na pozici ${n}`,
    fromPosition: (n: number) => `z pozice ${n}`,
    empty: 'Program zatím nemá žádné body.',
    emptyHint: 'Přidejte problémy ze zásobníku.',
    markDone: 'Vyřešeno',
    markOpen: 'Vrátit mezi nevyřešené',
    printContinued: (n: number, title: string) => `pokračování bodu ${n} — ${title}`,
    printLink: 'Odkaz:',
    printVideo: 'Videozáznam',
    printAttachment: 'Příloha ke stažení:',
  },

  picker: {
    title: 'Přidat problémy na program',
    subtitle: 'vybrané body se přidají na konec programu',
    onAgenda: 'Už na programu',
    doneHint: 'Vyřešené problémy se v seznamu nenabízejí.',
    cancel: 'Zrušit',
    submitEmpty: 'Přidat na program',
    submit: (n: number) => `Přidat ${n} ${plural(n, 'problém', 'problémy', 'problémů')} na program`,
    empty: 'V zásobníku nejsou žádné další problémy.',
  },

  confirm: {
    deleteProblemTitle: (title: string) => `Smazat problém „${title}“?`,
    deleteProblemBody: (n: number) =>
      `Problém se smaže i z ${n === 1 ? 'jedné porady' : `${czechNumeral(n)} porad`}, na ${n === 1 ? 'které' : 'kterých'} už byl. ` +
      `Zmizí z ${n === 1 ? 'jejího' : 'jejich'} programu včetně poznámek k přípravě a akcí:`,
    deleteProblemNoMeetings: 'Problém zatím nebyl na žádné poradě.',
    deleteProblemFinal: (attachments: number) =>
      attachments > 0
        ? `Smazání nelze vzít zpět. Smaže se i ${attachments} ${plural(attachments, 'příloha', 'přílohy', 'příloh')}.`
        : 'Smazání nelze vzít zpět.',
    deleteProblemConfirm: 'Smazat problém',
    deleteMeetingTitle: (date: string, week: string) => `Smazat poradu z ${date} (${week.toLowerCase()})?`,
    deleteMeetingBody: (n: number) =>
      `Smaže se program porady se všemi ${n} ${plural(n, 'bodem', 'body', 'body')}, poznámka k celé poradě i poznámky k přípravě a akce u jednotlivých bodů.`,
    deleteMeetingEmphasis: 'Samotné problémy se nesmažou',
    deleteMeetingTail: ' — vrátí se mezi nevyřešené do zásobníku.',
    deleteMeetingLink: (slug: string) => `Odkaz /porada/${slug} přestane fungovat. Smazání nelze vzít zpět.`,
    deleteMeetingConfirm: 'Smazat poradu',
    cancel: 'Zrušit',
  },

  lightbox: {
    close: 'Zavřít',
    prev: 'Předchozí',
    next: 'Další',
    download: 'Stáhnout',
    counter: (i: number, n: number) => `příloha ${i} z ${n}`,
  },

  errors: {
    network: 'Server neodpovídá. Zkontrolujte připojení k síti.',
    unknown: 'Došlo k neočekávané chybě.',
    notFoundTitle: 'Záznam nebyl nalezen.',
    notFoundMeeting: (slug: string) => `Adresa /porada/${slug} neexistuje nebo byla porada smazána.`,
    notFoundProblem: (id: string) => `Problém s číslem ${id} neexistuje nebo byl smazán.`,
    toMeetings: 'Přejít na seznam porad',
    toProblems: 'Přejít na seznam problémů',
    // True of every rejected submit, whatever refused it, so it is kept apart
    // from the remedies below: the reassurance that nothing was lost is worth
    // as much to a refused author as to an oversized file.
    notSaved: 'Problém se neuložil. Formulář i vložené přílohy zůstávají vyplněné.',
    // Checked in the browser before the submit is sent; the server enforces the
    // same three caps and its message wins if one ever slips through.
    fileTooLarge: (name: string, size: string) =>
      `Soubor ${name} má ${size} a je větší než povolených 100 MB.`,
    fileTooLargeHint: 'Odeberte soubor a uložte znovu.',
    tooManyFiles: (n: number, max: number) =>
      `Najednou lze nahrát nejvýš ${max} souborů. Vybráno ${n}.`,
    tooManyFilesHint: (over: number) =>
      `Odeberte ${over} ${plural(over, 'přílohu', 'přílohy', 'příloh')}, nebo je vložte do druhého problému.`,
    requestTooLarge: (total: string, max: string) =>
      `Přílohy dohromady mají ${total}, povoleno je ${max}.`,
    requestTooLargeHint:
      'Žádný soubor sám limit 100 MB nepřekročil — nevejde se jejich součet. Odeberte některé záznamy a uložte znovu.',
    titleRequired: 'Vyplňte název problému.',
    titleTooLong: 'Název může mít nejvýše 200 znaků.',
    linkScheme: 'Odkaz musí začínat http:// nebo https://.',
  },

  common: {
    loading: 'Načítám…',
    retry: 'Zkusit znovu',
  },
} as const

/**
 * Czech has three plural forms: 1, 2–4, and 5+.
 *
 * Getting this wrong is the most visible way an app reads as machine-written,
 * and every count in this UI is small enough for the rule to apply plainly.
 */
export function plural(n: number, one: string, few: string, many: string): string {
  if (n === 1) return one
  if (n >= 2 && n <= 4) return few
  return many
}

/** Spells the small numbers that read badly as digits in a sentence. */
function czechNumeral(n: number): string {
  const words = ['nula', 'jedné', 'dvou', 'tří', 'čtyř', 'pěti', 'šesti', 'sedmi', 'osmi', 'devíti', 'deseti']
  return words[n] ?? String(n)
}

export const problemCount = (n: number) => `${n} ${plural(n, 'problém', 'problémy', 'problémů')}`
export const itemCount = (n: number) => `${n} ${plural(n, 'bod', 'body', 'bodů')}`

/** The bucket's subtitle: "7 nevyřešených problémů". */
export function bucketCount(n: number, filter: 'open' | 'done' | 'all'): string {
  const noun = plural(n, 'problém', 'problémy', 'problémů')
  if (filter === 'all') return `${n} ${noun}`
  const adjective =
    filter === 'open'
      ? plural(n, 'nevyřešený', 'nevyřešené', 'nevyřešených')
      : plural(n, 'vyřešený', 'vyřešené', 'vyřešených')
  return `${n} ${adjective} ${noun}`
}
