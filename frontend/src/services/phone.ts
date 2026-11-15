import { aboutImages, groupNumber } from '../config/about'
export async function copyGroupNumber() { await navigator.clipboard.writeText(groupNumber) }
export async function saveAboutImage(kind: keyof typeof aboutImages) {
  const image = aboutImages[kind]
  const link = document.createElement('a')
  link.href = image.url
  link.download = image.filename
  document.body.appendChild(link)
  link.click()
  link.remove()
}
