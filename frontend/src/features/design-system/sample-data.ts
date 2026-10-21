// Realistic sample content for the design system showcase: KazUTB students and
// projects. Proper names stay as written; descriptions come from translations.

export const people = [
  { name: 'Айгерім Сапарова', online: true },
  { name: 'Нұрлан Әбдірахманов', online: true },
  { name: 'Даниял Ержанов', online: false },
  { name: 'Мадина Төлеген', online: false },
  { name: 'Ерасыл Қуанышев', online: false },
  { name: 'Әсем Жұмабаева', online: false },
  { name: 'Тимур Каримов', online: false },
];

export type SampleProjectId = 'agro' | 'tildes' | 'cargo';

export const sampleProjects: {
  id: SampleProjectId;
  name: string;
  score: number;
  team: string[];
  cover: string;
}[] = [
  {
    id: 'agro',
    name: 'AgroSense',
    score: 78,
    team: ['Айгерім Сапарова', 'Нұрлан Әбдірахманов', 'Даниял Ержанов'],
    cover: 'from-success-300 via-primary-400 to-primary-700',
  },
  {
    id: 'tildes',
    name: 'Тілдес',
    score: 71,
    team: ['Мадина Төлеген', 'Әсем Жұмабаева'],
    cover: 'from-primary-300 via-info-400 to-info-700',
  },
  {
    id: 'cargo',
    name: 'QazCargo Hub',
    score: 54,
    team: [
      'Ерасыл Қуанышев',
      'Тимур Каримов',
      'Айгерім Сапарова',
      'Даниял Ержанов',
      'Мадина Төлеген',
    ],
    cover: 'from-gold-300 via-warning-400 to-danger-500',
  },
];

export const leaderboard = [
  { rank: 1, project: 'AgroSense', faculty: 'it', score: 87.4, votes: 412 },
  { rank: 2, project: 'Тілдес', faculty: 'humanities', score: 84.9, votes: 388 },
  { rank: 3, project: 'EcoPack KZ', faculty: 'food', score: 81.2, votes: 256 },
  { rank: 4, project: 'QazCargo Hub', faculty: 'economics', score: 76.8, votes: 197 },
  { rank: 5, project: 'MedQueue', faculty: 'it', score: 73.5, votes: 164 },
] as const;

/** Monthly series for KPI sparklines (last 8 months). */
export const trends = {
  projects: [61, 68, 74, 72, 85, 93, 104, 118],
  students: [214, 236, 251, 247, 288, 316, 342, 371],
  showcase: [12, 14, 15, 19, 21, 24, 29, 33],
  score: [52, 54, 55, 57, 56, 59, 61, 63],
};
