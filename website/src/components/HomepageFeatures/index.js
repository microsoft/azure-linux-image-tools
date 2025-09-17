import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';
const { sprintf } = require('sprintf-js');

const FeatureList = [
  {
    title: 'Image Customizer',
    // Svg: require('@site/static/img/image_customizer.svg').default,
    to: './docs/imagecustomizer/',
    description: (
      <>
        Leverage existing generic Azure Linux images with Image Customizer to create customized images for your particular scenario.
      </>
    ),
  },
  {
    title: 'Image Creator',
    // Svg: require('@site/static/img/image_createor.svg').default,
    to: './docs/imagecreator/',
    description: (
      <>
        Build Azure Linux operating system images from scratch with Image Creator.
      </>
    ),
  },
];

function Feature({ Svg, title, description, to, featureRowClass }) {
  return (
    <div className={clsx(featureRowClass)}>
      <a href={to} className={styles.noUnderlineLink}>
        <div className="text--center padding-horiz--md">
          <Heading as="h3">{title}</Heading>
          <p>{description}</p>
        </div>
      </a>
    </div>
  );
}

export default function HomepageFeatures() {
  // There seem to be 12 columns for the feature list ... 'col--6` says each
  // feature gets 6 columns, so 2 per row ... the group of 2 would be centered.
  // If there were 1 feature, it would not be centered, but would look left
  // justified ... 1 could be centered if `col--12` was used.
  //
  // Do some work here to try to keep things centered-ish for various counts of
  // features.
  let featureRowClass = 'col col--3';
  if (FeatureList.length > 0 && FeatureList.length < 4) {
    featureRowClass = sprintf("col col--%d", 12 / (FeatureList.length))
  }

  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props}  featureRowClass={featureRowClass} />
          ))}
        </div>
      </div>
    </section>
  );
}
