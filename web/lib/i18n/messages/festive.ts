import { defineMessages } from "../define";

export const festive = defineMessages({
  vi: {
    festive: {
      turnOff: "Tắt hiệu ứng theo mùa",
      turnOn: "Bật hiệu ứng theo mùa",
      musicOn: "Bật nhạc nền",
      musicOff: "Tắt nhạc nền",
      nowPlaying: (title: string, composer: string) => `Đang phát: ${title} — ${composer}`,
      greeting: {
        newyear: "Chúc mừng năm mới",
        tet: "Chúc mừng năm mới — An khang thịnh vượng",
        valentine: "Valentine ngọt ngào",
        womensday: "Chúc mừng ngày Quốc tế Phụ nữ 8/3",
        midautumn: "Trung thu vui vẻ",
        halloween: "Halloween vui vẻ",
        christmas: "Giáng sinh an lành",
      },
    },
  },
  en: {
    festive: {
      turnOff: "Turn off seasonal effects",
      turnOn: "Turn on seasonal effects",
      musicOn: "Play background music",
      musicOff: "Stop background music",
      nowPlaying: (title: string, composer: string) => `Now playing: ${title} — ${composer}`,
      greeting: {
        newyear: "Happy New Year",
        tet: "Happy Lunar New Year",
        valentine: "Happy Valentine's Day",
        womensday: "Happy International Women's Day",
        midautumn: "Happy Mid-Autumn Festival",
        halloween: "Happy Halloween",
        christmas: "Merry Christmas",
      },
    },
  },
});
