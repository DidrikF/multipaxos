var path = require('path')
var webpack = require('webpack')

module.exports = {
  mode: "development",
  entry: {
    app: './src/app.js', 
    start: './src/start.js'
  },
  output: {
    path: __dirname + '/dist',
    publicPath: '/dist/',
    filename: '[name].js'
  },
  module: {
    rules: [
        {
            test: /\.css$/,
            use: [
                'style-loader', 'css-loader'
            ],
        },
        {
            test: /\.sass$/,
            use: [
                'style-loader',
                'css-loader',
                'sass-loader?indentedSyntax'
            ],
        },
        {
          test: /\.vue$/,
          loader: 'vue-loader',
          options: {
            loaders: {
              // Since sass-loader (weirdly) has SCSS as its default parse mode, we map
              // the "scss" and "sass" values for the lang attribute to the right configs here.
              // other preprocessors should work out of the box, no loader config like this necessary.
              sass: [
                'vue-style-loader',
                'css-loader',
                'sass-loader?indentedSyntax',
              ],
            },
            // other vue-loader options go here
          },
        },
        {
            test: /\.js$/,
            loader: 'babel-loader',
            exclude: /node_modules/
        },
        {
            test: /\.tsx?$/,
            loader: 'ts-loader',
            exclude: /node_modules/,
            options: {
                appendTsSuffixTo: [/\.vue$/],
            }
        },
    ]
  },
  resolve: {
    extensions: ['.ts', '.js', '.vue', '.json'],
    alias: {
      'vue$': 'vue/dist/vue.esm.js',
      styles: path.resolve(__dirname, 'src', 'styles'),
    }
  },
  devServer: {
    historyApiFallback: true,
    noInfo: true
  },
  performance: {
    hints: false
  },
  devtool: '#eval-source-map'
}

if (process.env.NODE_ENV === 'production') {
  module.exports.devtool = '#source-map'
  // http://vue-loader.vuejs.org/en/workflow/production.html
  module.exports.plugins = (module.exports.plugins || []).concat([
    new webpack.DefinePlugin({
      'process.env': {
        NODE_ENV: '"production"'
      }
    }),
    new webpack.optimize.UglifyJsPlugin({
      sourceMap: true,
      compress: {
        warnings: false
      }
    }),
    new webpack.LoaderOptionsPlugin({
      minimize: true
    })
  ])
}