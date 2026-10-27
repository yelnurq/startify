/** 0–4: length carries most of the weight, variety of characters the rest. */
export function passwordScore(password: string): number {
  if (password.length === 0) return 0;
  let score = 0;
  if (password.length >= 10) score++;
  if (password.length >= 14) score++;
  const classes = [/[a-zа-яәғқңөұүһі]/, /[A-ZА-ЯӘҒҚҢӨҰҮҺІ]/, /[0-9]/, /[^A-Za-zА-Яа-я0-9]/].filter(
    (re) => re.test(password),
  ).length;
  if (classes >= 3) score++;
  if (password.length >= 18 || (password.length >= 12 && classes === 4)) score++;
  return Math.min(score, 4);
}
