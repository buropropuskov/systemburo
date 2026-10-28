import { describe, expect, it } from 'vitest';
import { onboardingSteps } from '../onboardingSteps';
import { cabinetApplicationSteps } from '../cabinetApplicationSteps';
import { acceptCardSteps } from '../acceptCardSteps';
import { adminOnboardingSteps } from '../adminOnboardingSteps';
import { approveOnboardingSteps } from '../approveOnboardingSteps';
import { acceptOnboardingSteps } from '../acceptOnboardingSteps';
import { securityOnboardingSteps } from '../securityOnboardingSteps';
import { buildSecurityFactSteps, buildSecurityFinalStep } from '../securityFactSteps';

const step = (steps, id) => {
  const found = steps.find((item) => item.id === id);
  expect(found, id).toBeDefined();
  return found;
};

describe('смысловые обязательства текста обучения', () => {
  it('не обещает неизмеренную длительность прохождения', () => {
    for (const steps of [onboardingSteps, securityOnboardingSteps, approveOnboardingSteps, acceptOnboardingSteps, adminOnboardingSteps]) {
      expect(steps[0].description).not.toMatch(/займ[её]т.*минут/iu);
    }
  });

  it('не привязывает новости к двум всегда открытым вкладкам', () => {
    const copy = step(adminOnboardingSteps, 'admin-news').description;
    expect(copy).toMatch(/выберите нужный тип/iu);
    expect(copy).not.toMatch(/две вкладки|вместе с вкладкой/iu);
  });

  it('ролевые финалы не утверждают освоение закрытых действий', () => {
    for (const [steps, id] of [[approveOnboardingSteps, 'apr-finish'], [acceptOnboardingSteps, 'acc-finish']]) {
      const copy = step(steps, id).description;
      expect(copy).toMatch(/доступные вам разделы/iu);
      expect(copy).not.toMatch(/голосуете|назначаете|принимаете в работу/iu);
    }
  });

  it('отличает подтверждение подачи от личного согласия аккаунта', () => {
    const copy = step(onboardingSteps, 'createapp-consent');
    expect(copy.description).toMatch(/лица уведомлены об обработке персональных данных/iu);
    expect(copy.description).toMatch(/сведения переданы правомерно/iu);
    expect(copy.description).not.toMatch(/подтверждаете согласие|согласие на (обработку|хранение|передачу)/iu);
    // На телефоне документ открывается в окне, на широком экране - во вкладке.
    expect(copy.description).not.toMatch(/вкладк|модальн/iu);
  });

  it('сохраняет работу текущего устройства после собственной смены пароля', () => {
    const copy = step(onboardingSteps, 'cabinet-password').description;
    expect(copy).toMatch(/на этом устройстве.*продолжите работу/iu);
    expect(copy).toMatch(/на остальных.*войти с новым паролем/iu);
    expect(copy).not.toMatch(/после смены нужно будет войти заново/iu);
  });

  it('не обещает автоподстановку закрытых документов существующего сотрудника', () => {
    const copy = step(onboardingSteps, 'createapp-people-existing').description;
    expect(copy).toMatch(/доступные данные подставятся/iu);
    expect(copy).toMatch(/недостающие сведения о документах.*для этой заявки/iu);
    expect(copy).not.toMatch(/паспортные данные подставятся сами/iu);
  });

  it('различает новую копию и изменение срока исходной заявки', () => {
    const copy = step(cabinetApplicationSteps, 'detail-duplicate').description;
    expect(copy).toContain('«Продублировать / продлить»');
    expect(copy).toMatch(/создаёт новую заявку/iu);
    expect(copy).toMatch(/срок исходной заявки не изменится/iu);
    for (const label of ['На следующий месяц', 'На завтра', 'Другой срок']) {
      expect(copy).toContain(`«${label}»`);
    }
  });

  it('не гарантирует раскрытие настоящего имени принимающего', () => {
    const copy = step(acceptCardSteps, 'acc-detail-take').description;
    expect(copy).toMatch(/становитесь ответственным/iu);
    expect(copy).toMatch(/результат виден заявителю/iu);
    expect(copy).not.toMatch(/(имя|фамили[яю]|ФИО)/iu);
  });

  it('не обещает отключить обязательные уведомления', () => {
    const copy = step(onboardingSteps, 'notif-categories').description;
    expect(copy).toMatch(/обязательные.*нельзя отключить.*останутся включёнными/iu);
    expect(copy).not.toMatch(/без напоминаний/iu);
  });

  it('обзор и финал ограниченного администратора не перечисляют закрытые операции', () => {
    for (const id of ['admin-more', 'admin-finish']) {
      const copy = step(adminOnboardingSteps, id).description;
      expect(copy).toMatch(/доступн/iu);
      expect(copy).not.toMatch(/файловый архив|мониторинг запросов|доступ к перс|журнал отказов|правите.*руковод/iu);
    }
  });

  it('объясняет этап решения дополнения, не обещая кнопки в ожидающем раунде', () => {
    const copy = step(acceptCardSteps, 'acc-detail-supplement').description;
    expect(copy).toMatch(/после согласования станут доступны кнопки/iu);
    expect(copy).toMatch(/уже выданные пропуска продолжают действовать/iu);
  });

  it('основные блоки формы и карточки описаны без фиксированных двух колонок', () => {
    for (const id of ['createapp-selector', 'createapp-car-form', 'createapp-blank-switch', 'createapp-people-form']) {
      expect(step(onboardingSteps, id).description).not.toMatch(/слева|справа|две колонки/iu);
    }
    expect(step(acceptCardSteps, 'acc-detail-card').description).not.toMatch(/слева|справа|две колонки/iu);
  });

  it('сохраняет материал «По факту», а финалом не утверждает освоение пропущенных действий', () => {
    const facts = buildSecurityFactSteps('/tables/training-post');
    expect(step(facts, 'sec-fact-intro').description).toMatch(/по факту/iu);
    expect(buildSecurityFinalStep().description).not.toMatch(/вы освоились|отмечаете въезд|свериться с отчётом/iu);
    expect(step(onboardingSteps, 'finish').description).not.toMatch(/туры для других ролей/iu);
  });
});
